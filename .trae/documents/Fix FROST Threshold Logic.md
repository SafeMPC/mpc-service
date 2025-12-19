Based on your feedback that "3-of-3 works" but "2-of-3 fails" (which I've also observed), the issue is almost certainly related to **subset signing** in FROST.

**Analysis:**
1.  **3-of-3 Case (All parties sign)**:
    *   `parties` list contains 3 nodes.
    *   `tss-lib` parameter `threshold` is set to `len(parties)-1 = 2`.
    *   This matches the DKG threshold ($t=2$).
    *   Result: **Success**.

2.  **2-of-3 Case (Subset signing)**:
    *   The system selects 2 nodes to sign.
    *   `parties` list contains 2 nodes.
    *   `tss-lib` parameter `threshold` is set to `len(parties)-1 = 1`.
    *   This **mismatches** the DKG threshold ($t=2$).
    *   Result: **Failure**. `tss-lib` likely rejects the shares because they were generated with a polynomial of degree 2, but the signing session is initialized with degree 1. Or, the Lagrange interpolation fails because it expects $t+1$ shares to reconstruct the secret (conceptually), but the math requires the original polynomial degree $t$ to be consistent.

**The Root Cause:**
In `tss-lib`'s EdDSA (FROST) implementation, the `threshold` parameter passed to `NewParameters` during signing **must match the original DKG threshold ($t$)**, not the current number of signers minus 1.
However, `tss-lib` enforces `threshold <= len(parties) - 1`.
This implies that for FROST, you **cannot** do a subset signing with *exactly* $t+1$ parties if you naively set `threshold` to $t$.
Wait, if $t=2$ (3-of-3 key), you need 3 signers ($t+1$).
If you have 2 signers, you can only support $t=1$.
If the key was generated with $t=2$, you **must** have 3 signers to sign ($t+1$).
You **cannot** sign with 2 parties if $t=2$.

**Correction on "2-of-3" meaning:**
*   User input "Threshold: 2" usually means "2 signers required". This implies $t=1$.
*   If you created a key with "Threshold: 2" in our API:
    *   My fix in `executeKeygen` (for ECDSA) sets $t = \text{Threshold} - 1 = 1$.
    *   So $t=1$.
    *   Total nodes = 3.
    *   This is a valid 2-of-3 setup ($t=1, n=3$).
*   If we try to sign with 2 nodes:
    *   `len(parties) = 2`.
    *   My hardcoded logic sets `tssThreshold = len(parties) - 1 = 1`.
    *   This matches $t=1$.
    *   **This should work.**

**Why did it fail for you before?**
*   Maybe the key was created with "Threshold: 3" (meaning $t=2$)? No, you said "2of3".
*   If you used my previous logic (before I added the `-1` fix for ECDSA), "Threshold: 2" might have been passed as $t=2$ to `tss-lib`.
*   If $t=2$, then you need 3 signers.
*   If you try to sign with 2 signers, it's mathematically impossible (you need $t+1$ shares).

**Hypothesis:**
The FROST code path in `tss_adapter.go` (`executeEdDSAKeygen`) **does not** have the `threshold - 1` fix I applied to ECDSA.
Let's check `executeEdDSAKeygen`.

```go
func (m *tssPartyManager) executeEdDSAKeygen(...) {
    // ...
    ctxTSS := tss.NewPeerContext(parties)
    params := tss.NewParameters(tss.Edwards(), ctxTSS, thisPartyID, len(parties), threshold)
    // ...
}
```
**It passes `threshold` directly!**
If you call API with `threshold: 2` (meaning 2 signers), `executeEdDSAKeygen` receives `2`.
It sets `tss` threshold $t=2$.
$t=2$ requires $t+1=3$ signers.
So a "2-of-3" key (in your intent) becomes a "3-of-3" key (mathematically).
When you try to sign with 2 nodes:
*   You are trying to reconstruct a degree-2 polynomial with only 2 points.
*   Impossible.
*   `tss-lib` hangs or errors out.

**The Fix:**
I need to apply the same `threshold - 1` logic to `executeEdDSAKeygen` as I did for `executeKeygen` (ECDSA).
Then "Threshold: 2" will result in $t=1$, which requires $1+1=2$ signers.
Then signing with 2 nodes will work.

I will implement this fix now.