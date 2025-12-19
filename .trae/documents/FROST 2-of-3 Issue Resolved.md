The FROST "2-of-3" signing issue has been successfully resolved.

**Root Cause:**
*   The `threshold` parameter logic in `executeEdDSAKeygen` (FROST) was inconsistent with `executeKeygen` (ECDSA).
*   When you requested "Threshold: 2" (meaning 2 signers):
    *   FROST KeyGen used $t=2$ (requiring 3 signers).
    *   ECDSA KeyGen used $t=1$ (requiring 2 signers).
*   Consequently, attempting to sign with only 2 nodes failed for FROST because the key mathematically required 3 shares ($t+1=3$).

**Fix Implemented:**
*   Applied the `threshold - 1` logic to `executeEdDSAKeygen` in `tss_adapter.go`.
*   Now, "Threshold: 2" correctly sets the polynomial degree $t=1$.
*   This allows a valid signature to be reconstructed with 2 participants ($t+1=2$).

**Verification:**
*   **Key Generation**: Created a new FROST Root Key with `threshold=2, total_nodes=3`.
*   **Signing**: Successfully signed message "hello threshold" using only 2 participating nodes (`server-proxy-1`, `server-proxy-2`).
    *   Result: `{"signature":"2e5374b5...","status":"Active"}`.

**Conclusion:**
FROST (EdDSA) Root Key signing now correctly supports subset signing (e.g., 2-of-3) and is fully functional for standard signing operations. Note that key derivation is still not supported for EdDSA.