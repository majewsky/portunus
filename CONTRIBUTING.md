# CONTRIBUTING.md

## Notes for developers

When editing files in `static/`, [sassc](https://github.com/sass/sassc) is
required to compile the SCSS files in `static/css/` into CSS.

After editing CSS files, always commit the generated file
`static/css/portunus.css`. This ensures that the next person can `go build`
without these dependencies.
