# Per-user MagicDNS production guide

## Behavior

With `dns.per_user_subdomains: true`:

- Untagged nodes are named only `<node>.<user>.<baseDomain>`.
- Tagged nodes keep `<node>.<baseDomain>`, including nodes with creator metadata.
- CLI users use their normalized account name. OIDC users also receive a stable
  `-<userID>` suffix so duplicate usernames cannot collide.
- Missing users receive `<node>.orphan-<nodeID>.<baseDomain>` instead of
  breaking unrelated network maps.
- User renames update connected clients and remove the old alias.

The setting defaults to `false`, which preserves stock names. Bare `<node>` is
not resolved in per-user mode; use the complete FQDN. ACLs and tailnet routes
do not change.

## Pinned package

`packaging/per-user-magicdns.nix` currently pins:

- Revision: `147666ce48cdeeda69e51f96de7f0fc5db979e41`
- Source hash: `sha256-EYIdCiVQqByt3oxQy11bCkAIPrPQJi/RJcv3W+xbkTg=`

After publishing the fork, change the source URL from the local verification
path to the reviewed remote fork URL without changing the reviewed revision or
hash. Re-run `nix-prefetch-git` whenever either changes.

Import the package in Wolfra_IT with:

```nix
perUserHeadscale = pkgs.callPackage ./packaging/per-user-magicdns.nix { };
```

The Go module dependencies did not change, so upstream's vendor hash is
retained.

## Build verification

Apply the package override in a disposable Wolfra_IT environment and leave the
setting disabled. Run exactly:

```sh
nix build --no-link --no-update-lock-file --no-write-lock-file \
  '.#nixosConfigurations.hub.config.system.build.toplevel'
```

Record the resulting immutable `/nix/store/...-nixos-system-hub-...` path. This
verifies the default-off configuration without changing production. The
verified closure for the pinned revision is:

```text
/nix/store/phrji1dpbbczndnhlkr0x6k5h9n75g0x-nixos-system-hub-26.05.20261015.26f4107
```

## Rollback rehearsal

In the disposable environment:

1. Boot the default-off closure and verify stock `<node>.<baseDomain>` names.
2. Set `services.headscale.settings.dns.per_user_subdomains = true` and verify
   untagged per-user names, tagged names, and old-alias removal.
3. Restore the recorded previous closure and verify stock behavior returns.
4. Confirm no Headscale database migration was required.

Do not perform production activation as part of this rehearsal.

## Production checklist

Production activation requires separate explicit approval. Before approval:

- Back up Headscale using the existing Hub backup service and verify the backup
  is restorable.
- Record the current system closure for rollback.
- Review untagged services' TLS names and issue replacement certificates if
  needed. Tagged services such as Jellyfin keep their existing names and certs.
- Enable the setting and verify connected clients, node deletion/recreation,
  user renames, tagged nodes, and old-alias absence.
- If validation fails, restore the recorded closure and previous configuration.

## Live validation

Build the exact-revision integration image, then run:

```sh
HEADSCALE_INTEGRATION_HEADSCALE_IMAGE=headscale-integration:prototype \
  go run ./cmd/hi run 'TestResolveMagicDNS$'
HEADSCALE_INTEGRATION_HEADSCALE_IMAGE=headscale-integration:prototype \
  go run ./cmd/hi run 'TestResolveTaggedMagicDNS$'
HEADSCALE_INTEGRATION_HEADSCALE_IMAGE=headscale-integration:prototype \
  go run ./cmd/hi run 'TestResolveMagicDNSDefaultOff$'
HEADSCALE_INTEGRATION_HEADSCALE_IMAGE=headscale-integration:prototype \
  go run ./cmd/hi run 'TestMagicDNSLifecycle$'
```

Also require `go test ./...` and changed-file `golangci-lint` to pass on
`x86_64-linux` before publishing a release candidate.
