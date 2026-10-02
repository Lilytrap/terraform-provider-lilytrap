# terraform-provider-lilytrap

Terraform provider for [Lilytrap](https://lilytrap.com) decoys.
Docs are in [docs/](docs/). An example is in [examples/main.tf](examples/main.tf).

## Develop

```sh
go test ./...
go build -o ~/.terraform.d/dev/terraform-provider-lilytrap .
```

Point Terraform at the build with a `dev_overrides` block in `~/.terraformrc`.

## Release

1. Add `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` secrets to this repo.
2. Add the public key in the Terraform Registry under your namespace's signing keys.
3. Push a `v*` tag. The release workflow builds and signs with GoReleaser.
4. Publish the repo once in the Registry. Later tags are picked up automatically.
