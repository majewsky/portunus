# CONTRIBUTING.md

## Notes for developers

When editing files in `static/`, some additional software is required:

- [sassc](https://github.com/sass/sassc) for compiling the SCSS files in `static/css/` into CSS
- [go-bindata](https://github.com/shuLhan/go-bindata) for packing the files in `static/` into the binary

After editing files in `static/`, always commit the generated files
`internal/static/bindata.go` and `static/css/portunus.css`. This ensures that
the next person can `go build` without these dependencies.
