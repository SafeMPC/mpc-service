package protocol

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/kashguard/tss-lib/crypto"
	"github.com/kashguard/tss-lib/ecdsa/keygen"
	eddsaKeygen "github.com/kashguard/tss-lib/eddsa/keygen"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// parseDerivationPath parses a BIP-32 derivation path string into indices
// Example: "m/44'/60'/0'/0/0" -> [44+H, 60+H, 0+H, 0, 0]
// Note: In MPC, we only support non-hardened derivation for now.
func parseDerivationPath(path string) ([]uint32, error) {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return nil, errors.New("empty derivation path")
	}
	if parts[0] == "m" {
		parts = parts[1:]
	}

	indices := make([]uint32, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		var index uint32
		var err error

		isHardened := false
		if strings.HasSuffix(part, "'") || strings.HasSuffix(part, "h") || strings.HasSuffix(part, "H") {
			isHardened = true
			part = part[:len(part)-1]
		}

		val, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid path component: %s", part)
		}
		index = uint32(val)

		if isHardened {
			index |= 0x80000000
		}
		indices = append(indices, index)
	}
	return indices, nil
}

// computeIL calculates the Intermediate Value IL and Child Chain Code for a given index
// Note: This duplicates logic from key/derivation.go to avoid import cycles.
func computeIL(pubKey *btcec.PublicKey, chainCode []byte, index uint32) (*big.Int, []byte, error) {
	if index >= 0x80000000 {
		return nil, nil, errors.New("hardened derivation is not supported in MPC (requires private key reconstruction)")
	}

	if len(chainCode) != 32 {
		return nil, nil, errors.New("invalid chain code length: must be 32 bytes")
	}

	parentPubKeyBytes := pubKey.SerializeCompressed()
	hmac512 := hmac.New(sha512.New, chainCode)
	hmac512.Write(parentPubKeyBytes)

	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, index)
	hmac512.Write(indexBytes)

	I := hmac512.Sum(nil)
	IL := I[:32]
	IR := I[32:]

	ilNum := new(big.Int).SetBytes(IL)
	if ilNum.Cmp(btcec.S256().N) >= 0 || ilNum.Sign() == 0 {
		return nil, nil, errors.New("invalid derived key (IL >= n or IL = 0)")
	}

	return ilNum, IR, nil
}

// DeriveLocalPartySaveData derives a child key share from a parent key share
func DeriveLocalPartySaveData(parentData *keygen.LocalPartySaveData, parentChainCode []byte, derivationPath string) (*keygen.LocalPartySaveData, error) {
	if derivationPath == "" {
		return parentData, nil
	}

	indices, err := parseDerivationPath(derivationPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse derivation path")
	}

	if len(indices) == 0 {
		return parentData, nil
	}

	// Deep copy relevant parts of parentData
	childData := new(keygen.LocalPartySaveData)
	childData.LocalPreParams = parentData.LocalPreParams
	childData.Ks = parentData.Ks
	childData.ShareID = parentData.ShareID
	childData.PaillierPKs = parentData.PaillierPKs
	childData.NTildej = parentData.NTildej
	childData.H1j = parentData.H1j
	childData.H2j = parentData.H2j

	childData.Xi = new(big.Int).Set(parentData.Xi)
	childData.BigXj = make([]*crypto.ECPoint, len(parentData.BigXj))
	childData.ECDSAPub = nil

	// Copy BigXj
	for i, pt := range parentData.BigXj {
		childData.BigXj[i] = pt // We will create new points, but initially copy reference (or should we clone?)
		// Better clone points to be safe
		if pt != nil {
			childData.BigXj[i], _ = crypto.NewECPoint(pt.Curve(), pt.X(), pt.Y())
		}
	}

	// Initial state
	currentChainCode := parentChainCode
	curve := btcec.S256()

	// We can reconstruct the public key point
	currentX := parentData.ECDSAPub.X()
	currentY := parentData.ECDSAPub.Y()

	// Iterate
	for _, index := range indices {
		// 1. Prepare Public Key for computeIL
		// We need compressed public key
		var pkBytes []byte
		if currentY.Bit(0) == 0 {
			pkBytes = append([]byte{0x02}, currentX.Bytes()...)
		} else {
			pkBytes = append([]byte{0x03}, currentX.Bytes()...)
		}
		// Padding if needed (32 bytes X)
		if len(currentX.Bytes()) < 32 {
			padded := make([]byte, 32)
			copy(padded[32-len(currentX.Bytes()):], currentX.Bytes())
			pkBytes = append(pkBytes[:1], padded...)
		}

		btcecPubKey, err := btcec.ParsePubKey(pkBytes)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse public key")
		}

		// 2. Compute IL
		ilNum, nextChainCode, err := computeIL(btcecPubKey, currentChainCode, index)
		if err != nil {
			return nil, errors.Wrap(err, "compute IL failed")
		}

		// 🔍 [DIAGNOSTIC] Log IL for consistency check
		log.Info().
			Uint32("index", index).
			Str("il_hex", hex.EncodeToString(ilNum.Bytes())).
			Str("chain_code_hex", hex.EncodeToString(currentChainCode)).
			Str("next_chain_code_hex", hex.EncodeToString(nextChainCode)).
			Msg("🔍 [DIAGNOSTIC] DeriveLocalPartySaveData: IL Computed")

		// 3. Update Private Share (Xi) and Public Shares (BigXj)
		// For Shamir Secret Sharing (SSS), to add a constant IL to the secret (x' = x + IL),
		// we must add IL to EVERY share: xi' = xi + IL.
		// This ensures that any reconstructed secret (via Lagrange interpolation) includes IL,
		// because Sum(lambda_i * (xi + IL)) = Sum(lambda_i * xi) + IL * Sum(lambda_i) = x + IL.

		if len(childData.Ks) == 0 {
			return nil, errors.New("Ks is empty in LocalPartySaveData")
		}

		// Update Xi (Everyone does this)
		childData.Xi.Add(childData.Xi, ilNum)
		childData.Xi.Mod(childData.Xi, curve.N)

		// Update Public Shares (BigXj)
		// Xj' = Xj + IL*G for all j
		// Calculate Delta = IL * G
		// Ensure ilNum bytes are properly padded to 32 bytes for ScalarBaseMult
		ilBytes := ilNum.Bytes()
		if len(ilBytes) < 32 {
			padded := make([]byte, 32)
			copy(padded[32-len(ilBytes):], ilBytes)
			ilBytes = padded
		}
		deltaX, deltaY := curve.ScalarBaseMult(ilBytes)

		// 🔍 [DIAGNOSTIC] Log Delta
		log.Info().
			Str("delta_x", hex.EncodeToString(deltaX.Bytes())).
			Str("delta_y", hex.EncodeToString(deltaY.Bytes())).
			Msg("🔍 [DIAGNOSTIC] DeriveLocalPartySaveData: Delta Computed")

		// Update Global Public Key (Everyone does this)
		newX, newY := curve.Add(currentX, currentY, deltaX, deltaY)
		currentX = newX
		currentY = newY

		// Update All Public Shares (BigXj)
		for k, pt := range childData.BigXj {
			if pt == nil {
				continue
			}
			bx, by := curve.Add(pt.X(), pt.Y(), deltaX, deltaY)
			newPt, err := crypto.NewECPoint(curve, bx, by)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new ECPoint")
			}
			childData.BigXj[k] = newPt
		}

		// Update Chain Code for next iteration
		currentChainCode = nextChainCode
	}

	// Finalize ECDSAPub
	newPub, err := crypto.NewECPoint(btcec.S256(), currentX, currentY)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create final public key point")
	}
	childData.ECDSAPub = newPub

	// 4. Invariant Check: Xi * G == BigXj[myIndex]
	// This ensures our update logic maintained consistency for the local party.

	// Identify my index in Ks for verification
	myIndex := -1
	for i, k := range childData.Ks {
		if k.Cmp(childData.ShareID) == 0 {
			myIndex = i
			break
		}
	}

	if myIndex >= 0 && myIndex < len(childData.BigXj) {
		xiX, xiY := curve.ScalarBaseMult(childData.Xi.Bytes())
		bigXj := childData.BigXj[myIndex]
		if bigXj.X().Cmp(xiX) != 0 || bigXj.Y().Cmp(xiY) != 0 {
			log.Error().
				Int("my_index", myIndex).
				Msg("❌ [DIAGNOSTIC] DeriveLocalPartySaveData: Invariant Failed! Xi*G != BigXj[myIndex]")
			return nil, errors.New("derivation invariant failed")
		} else {
			log.Info().
				Int("my_index", myIndex).
				Msg("✅ [DIAGNOSTIC] DeriveLocalPartySaveData: Invariant Check Passed")
		}
	}

	return childData, nil
}

// computeIL_Ed25519 computes IL for Ed25519
func computeIL_Ed25519(pubKeyBytes []byte, chainCode []byte, index uint32) (*big.Int, []byte, error) {
	if index >= 0x80000000 {
		return nil, nil, errors.New("hardened derivation is not supported in MPC")
	}

	if len(chainCode) != 32 {
		return nil, nil, errors.New("invalid chain code length: must be 32 bytes")
	}

	// For Ed25519, pubKeyBytes is already 32 bytes (compressed y)
	hmac512 := hmac.New(sha512.New, chainCode)
	hmac512.Write(pubKeyBytes)

	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, index)
	hmac512.Write(indexBytes)

	I := hmac512.Sum(nil)
	IL := I[:32]
	IR := I[32:]

	ilNum := new(big.Int).SetBytes(IL)
	// Check against Ed25519 order
	// Note: For Ed25519, we skip the check because order L ~ 2^252 and we use modulo reduction.
	/*
		if ilNum.Cmp(edwards.Edwards().N) >= 0 || ilNum.Sign() == 0 {
			return nil, nil, errors.New("invalid derived key (IL >= n or IL = 0)")
		}
	*/
	if ilNum.Sign() == 0 {
		return nil, nil, errors.New("invalid derived key (IL = 0)")
	}

	return ilNum, IR, nil
}

// DeriveEdDSALocalPartySaveData derives a child key share from a parent key share (EdDSA)
func DeriveEdDSALocalPartySaveData(parentData *eddsaKeygen.LocalPartySaveData, parentChainCode []byte, derivationPath string) (*eddsaKeygen.LocalPartySaveData, error) {
	if derivationPath == "" {
		return parentData, nil
	}

	indices, err := parseDerivationPath(derivationPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse derivation path")
	}

	if len(indices) == 0 {
		return parentData, nil
	}

	// Deep copy
	childData := new(eddsaKeygen.LocalPartySaveData)
	// childData.LocalPreParams = parentData.LocalPreParams // EdDSA does not have PreParams
	childData.Ks = parentData.Ks
	childData.ShareID = parentData.ShareID // *big.Int, immutable? usually yes.

	childData.Xi = new(big.Int).Set(parentData.Xi)
	if parentData.EDDSAPub != nil {
		childData.EDDSAPub, _ = crypto.NewECPoint(parentData.EDDSAPub.Curve(), parentData.EDDSAPub.X(), parentData.EDDSAPub.Y())
	}

	// Clone BigXj (Public Shares)
	childData.BigXj = make([]*crypto.ECPoint, len(parentData.BigXj))
	for i, pt := range parentData.BigXj {
		if pt != nil {
			childData.BigXj[i], _ = crypto.NewECPoint(pt.Curve(), pt.X(), pt.Y())
		}
	}

	currentChainCode := parentChainCode
	curve := edwards.Edwards()

	// Reconstruct current Public Key point
	// tss-lib EDDSAPub is *crypto.ECPoint, which holds X, Y *big.Int
	currentX := childData.EDDSAPub.X()
	currentY := childData.EDDSAPub.Y()

	for _, index := range indices {
		// 1. Prepare Public Key bytes for computeIL
		// Ed25519 public key serialization: (y-coordinate | sign-bit-of-x)
		// dcrec/edwards/v2 PublicKey.Serialize() does this.
		pubKeyObj := edwards.PublicKey{
			Curve: curve,
			X:     currentX,
			Y:     currentY,
		}
		pubKeyBytes := pubKeyObj.Serialize()

		// 2. Compute IL
		ilNum, nextChainCode, err := computeIL_Ed25519(pubKeyBytes, currentChainCode, index)
		if err != nil {
			return nil, errors.Wrap(err, "compute IL failed")
		}

		// 3. Update Xi
		childData.Xi.Add(childData.Xi, ilNum)
		childData.Xi.Mod(childData.Xi, curve.N)

		// 4. Update Public Key (EDDSAPub)
		// P' = P + IL * G
		// Ensure ilNum bytes are properly padded to 32 bytes for ScalarBaseMult
		ilBytes := ilNum.Bytes()
		if len(ilBytes) < 32 {
			padded := make([]byte, 32)
			copy(padded[32-len(ilBytes):], ilBytes)
			ilBytes = padded
		}
		ilx, ily := curve.ScalarBaseMult(ilBytes)
		newX, newY := curve.Add(currentX, currentY, ilx, ily)

		currentX = newX
		currentY = newY

		// 5. Update All Public Shares (BigXj)
		// Xj' = Xj + IL*G
		for k, pt := range childData.BigXj {
			if pt == nil {
				continue
			}
			bx, by := curve.Add(pt.X(), pt.Y(), ilx, ily)
			newPt, err := crypto.NewECPoint(curve, bx, by)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new ECPoint for BigXj")
			}
			childData.BigXj[k] = newPt
		}

		currentChainCode = nextChainCode
	}

	// Finalize EDDSAPub
	newPub, err := crypto.NewECPoint(parentData.EDDSAPub.Curve(), currentX, currentY)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create final public key point")
	}
	childData.EDDSAPub = newPub

	return childData, nil
}
