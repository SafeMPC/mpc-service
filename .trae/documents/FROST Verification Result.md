FROST Root Key signing has been verified, but there is a persistent timeout issue that requires deeper investigation.

**Current Status:**
*   **Key Generation (EdDSA/FROST)**: ✅ Success. Root keys are created successfully.
*   **Key Derivation**: ❌ Not supported (BIP-32 Ed25519 not implemented).
*   **Signing (Root Key)**: ⚠️ **Failed/Timeout**.
    *   The signing process starts, but eventually times out (`FROST signing timeout`).
    *   Error logs show: `Failed to update EdDSA signing party from message ... tss_error={}`.
    *   This suggests an issue within `tss-lib`'s EdDSA signing logic or message handling compatibility between nodes.
    *   I attempted to fix a potential threshold parameter mismatch (`threshold` vs `len(parties)-1`), but it did not resolve the issue immediately, and reverts were necessary to maintain stability for other protocols.

**Conclusion:**
While **GG18** and **GG20** (ECDSA) are fully functional for both root and derived keys, **FROST (EdDSA)** signing is currently **unstable** in this environment. The root key can be generated, but signing operations are timing out.

**Recommendation:**
For now, please rely on **GG18/GG20** for production signing. FROST support requires further debugging of the underlying `tss-lib` integration or network message handling for EdDSA rounds. I recommend marking FROST as "Experimental/Unstable" until this timeout issue is resolved in a dedicated debugging session.