TODO make readable

- requires sassc for `static/css/`
- requires [go-bindata](https://github.com/shuLhan/go-bindata) for `static/`
- always commit `internal/static/bindata.go` and `static/css/portunus.css` after editing `static/`

## Example database

If you don't want to start from zero, copy the file `examples/database.json` to
`/var/lib/portunus/database.json`. The passwords for the users in that file are:

| User ID | Password |
| ------- | -------- |
| `jane` | `password` |
| `john` | `12345` |
