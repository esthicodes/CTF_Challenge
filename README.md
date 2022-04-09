# 🅿️🅿️🅿️ordle

### Requirements

* Go 1.18: https://go.dev/doc/install

## Usage

Navigate to the `client/` directory and run:
```bash
go run .
```

Can you beat all 4 levels?!

### Troubleshooting

* 🅿️🅿️🅿️ordle is best enjoyed on a screen with plenty of real estate, please consider using a monitor from one of our preferred partners (or try zooming in/out)
* Colors seem off? To make sure you're getting the most out of our state-of-the-art color system, enable True Color™️ in your terminal (e.g., `export TERM=screen-256color`)

### Development Environment

Set the `PPPORDLE_ENV` variable to `dev`:
```bash
export PPPORDLE_ENV=dev
```
or inline:
```bash
PPPORDLE_ENV=dev go run .
```

The server and client can be run locally from the `server/` and `client/` directories respectively with:

```bash
go run .
```
