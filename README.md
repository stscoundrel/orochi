# Orochi

Monitor certain firearms exercises on a certain island. When new events are detected, sends alert to sign up.

Descendant of now deprecated [Travers](https://github.com/stscoundrel/travers)


## Development

Requires Go 1.26

```sh
go build ./...
go vet ./...
go test -race -cover ./...
```

Tests use local fixtures, HTTP test servers, and fake repositories/object
storage.

## Deploy

For first-time setup, follow the [infrastructure instructions](infra/README.md).
Run from the repository root:

```sh
./scripts/package.sh
terraform -chdir=infra plan -out=orochi.tfplan
```

Review the plan, then apply it:

```sh
terraform -chdir=infra apply orochi.tfplan
```

## Whats in the name?

Orochi is japanese for "great serpent". Inspired by Kitaro track of the same name.