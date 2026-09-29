# Personal Center Design QA

## Comparison target

- Source visual truth: the user-supplied personal-center reference image in this conversation (2048 × 1014 rendered preview; original 2740 × 1356).
- Additional source visual truth: the user-supplied role-permission editor reference image in this conversation (1800 × 1200 rendered preview).
- Intended implementation: `/#/enterprise/personal-profile` in API mode, with the account menu open from the header avatar for the menu interaction state.
- Intended viewport: desktop 1366 × 768 and mobile 390 × 844, light theme, authenticated tenant member.

## Evidence status

- Source image: available in the conversation.
- Implementation screenshot: unavailable. The in-app Browser runtime has no available browser instance, and the local Playwright Chromium installation cannot run on this macOS version.
- Density normalization: not applicable without a browser-rendered implementation capture.
- Full-view and focused-region comparison: blocked; no visual comparison was performed from source code or memory.

## Implemented comparison intent

- The page hierarchy was changed to welcome area, basic-information card, security settings, and notification preferences.
- The basic-information card uses the account avatar and a three-column desktop facts grid, collapsing responsively on smaller screens.
- The left enterprise navigation no longer lists Personal Center. The header avatar opens a hover- and click-accessible menu containing Personal Center, Version Changes, and Sign Out.
- Version Changes opens the existing subscription change flow rather than a new placeholder page.
- The account menu remains open after pointer movement and closes on an outside pointer click, Escape, or menu-item selection; the Personal Center password action opens a password-only dialog.
- The role editor now presents a left permission tree and a right selected-permission panel. The right panel groups each currently selected permission by business domain and updates immediately as selection changes.
- The role editor no longer exposes a role-description field. Both permission panes are capped at 480px and use independent internal scrolling when their tree or selection list exceeds that height.
- The role editor offers total and assigned-permission counts plus an `All / Assigned` filter; the assigned filter retains only the current role's permission branches and exposes a recovery action when none are assigned.

## Findings

- [P1] Rendered fidelity cannot be judged.
  Evidence: no browser-rendered screenshot exists for either required viewport or the open account-menu state.
  Impact: typography, spacing, token contrast, avatar treatment, menu placement, responsive overflow, and screenshot fidelity to the source remain unverified.
  Fix: run the API-mode personal-profile fixture with a supported Chromium/browser, capture the reference-sized desktop and mobile views plus the open account menu, then compare those captures against the supplied reference.

## Implementation checklist

1. Acquire a supported browser runtime.
2. Capture `/enterprise/personal-profile` at 1366 × 768 and 390 × 844 in the same light state.
3. Capture the header avatar hover/click menu state and the new/edit-role dialog with selected permissions.
4. Compare composition, typography, spacing, tokens, icon/asset fidelity, and copy against both supplied references; resolve P0–P2 differences.

## Follow-up polish

- Determine whether the supplied illustrated avatar should be represented by an approved account-avatar asset for this product, or whether the existing dynamic account avatar is the intended product-specific deviation.

final result: blocked
