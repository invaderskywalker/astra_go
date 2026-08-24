# Astra documentation

These documents describe the behavior that should remain true as Astra evolves.

| Document | Purpose |
| --- | --- |
| [Capability map](capabilities.md) | Current abilities, implementation boundaries, evidence, and roadmap |
| [CLI cockpit](cli-cockpit.md) | Layout, keyboard controls, storage views, and workspace-access recovery |
| [Evaluations](evaluations.md) | Deterministic local gate and provider-backed evaluation contract |

## Source of truth

- Go action handlers define what Astra can actually do.
- Prompt policy and skills define how the executor should choose and verify work.
- Session, run, artifact, and Mind Palace files provide inspectable evidence.
- This documentation describes those contracts; it does not grant authority or
  replace runtime checks.

When adding a capability, update the implementation, add a deterministic test,
add or revise its evaluation scenario, and update the capability map in the same
change.
