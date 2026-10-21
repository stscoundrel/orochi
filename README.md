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

## Whats in the name?

Orochi is japanese for "great serpent". Inspired by Kitaro track of the same name.