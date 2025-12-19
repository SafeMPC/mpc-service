The signing issue has been successfully resolved!

**Summary of Fixes:**

1.  **Fixed `PartyID` Mapping**: Resolved the mismatch between `tss-lib` session IDs and stored key indices in `tss_adapter.go` by correctly using `keyData.Ks` for mapping.
2.  **Fixed Key Derivation Logic**:
    *   Implemented `keyDataWrapper` to fix `Ks` serialization loss.
    *   Corrected `DeriveLocalPartySaveData` to update all shares (`Xi` and `BigXj`) consistently and added padding to `ScalarBaseMult`.
    *   Verified consistency via Invariant Checks (`Xi*G == BigXj`).
3.  **Fixed Threshold Parameter Mismatch (Critical)**:
    *   Identified that `req.Threshold=2` (user input "2-of-3") was being interpreted as $t=2$ (requiring 3 signers), but only 2 signers were available.
    *   Modified `executeKeygen` in `tss_adapter.go` to use `threshold - 1` (converting "min signers" to "polynomial degree $t$"), matching the standard MPC expectation.
4.  **Increased Timeout**: Extended signing timeout to 5 minutes to accommodate emulated/slow environments (actual execution took ~20s).

**Verification:**
*   **Key Generation**: Created new Root Key (`threshold=2` input -> $t=1$).
*   **Key Derivation**: Successfully derived child key (`wallet-137c...`).
*   **Signing**: Successfully signed message "hello threshold" with the **Derived Key**.
    *   Response: `{"signature":"3044022037...","status":"Active"}`.

You can now proceed with using the wallet for signing operations.