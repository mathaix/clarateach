module github.com/clarateach/backend

go 1.24.0

toolchain go1.24.6

require (
	github.com/docker/docker v24.0.7+incompatible
	github.com/docker/go-connections v0.6.0
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-chi/cors v1.2.2
	github.com/mattn/go-sqlite3 v1.14.32
)

require (
	github.com/Microsoft/go-winio v0.4.21 // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/morikuni/aec v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v2.8.2+incompatible
	github.com/docker/docker => github.com/docker/docker v24.0.7+incompatible
)
