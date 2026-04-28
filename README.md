# MyPage

Personal software-engineering portfolio. Single Go binary, server-rendered HTML
with HTMX for interactivity. Dark theme, violet accents, mono typography.

## Stack

- **Backend:** Go stdlib `net/http` (1.22+ pattern routing). No framework.
- **Templates:** `html/template`, baked into the binary with `embed.FS`.
- **Frontend:** HTMX 2.0 + Tailwind via CDN. No build step.
- **Fonts:** JetBrains Mono + Space Grotesk (Google Fonts).

Everything ships in one self-contained executable — no external assets, no
template directory at runtime.

## Layout

```
MyPage/
├── go.mod
├── main.go                          # server + all portfolio content (in buildData)
├── templates/
│   ├── index.html                   # full page
│   └── experience-detail.html       # HTMX-swapped partial
└── static/
    ├── css/style.css                # custom styles + animations
    ├── js/                          # (empty — reserved)
    └── img/
        └── logo-placeholder.svg     # placeholder company logo
```

## Run it

Requires Go 1.22 or newer.

```sh
cd /home/chinmay/projects/MyPage
go build ./...
./mypage                       # serves on :8080
./mypage -addr=:9000           # pick a different port
```

Open http://localhost:8080. To stop, Ctrl-C.

For development you can skip the build step:

```sh
go run .
```

The binary is fully self-contained — `templates/` and `static/` are embedded at
compile time. You can copy just the `mypage` binary to a server and run it.

## Routes

| Method | Path                  | Purpose                                          |
|--------|-----------------------|--------------------------------------------------|
| GET    | `/`                   | Full portfolio page                              |
| GET    | `/experience/{id}`    | HTMX partial — work-experience detail card       |
| GET    | `/static/*`           | Embedded CSS / JS / images                       |

## Editing your content

All page content lives in **one place**: the `buildData()` function in
`main.go`. Every editable field has a `// EDIT —` comment next to it. Areas to
update:

- `Name`, `Handle`, `Tagline`, `Location`, `About`, `Interests` — hero + about.
- `Socials` — LinkedIn, Instagram, email URLs and handle text.
- `Experiences` — work history. Each entry becomes a clickable tab. To use a
  real company logo, drop a file into `static/img/` and set
  `LogoSrc: "/static/img/yourfile.svg"`. `ID` must be a unique URL slug.
- `Projects` — featured project cards (replace `https://github.com/` URLs).
- `GitHubURL` — powers the nav button and the bottom CTA.

To add a new social service (e.g. Twitter/X, Bluesky), define a new SVG icon
constant near the bottom of `main.go` (look for `iconLinkedIn`, `iconInstagram`,
`iconMail`) and reference it in the `Socials` slice.

After editing, rebuild:

```sh
go build ./... && ./mypage
```

The cached response bodies are rebuilt at every server start — no warm-up step.

## Performance notes

The server pre-renders every page (and gzips it) once at startup, then serves
the resulting byte slice directly:

- gzip via `compress/gzip` — picked when `Accept-Encoding: gzip` is set.
- ETag + `If-None-Match` → 304 Not Modified for repeat visitors.
- `Cache-Control: public, max-age=300` on dynamic responses, `max-age=86400` on
  static assets.
- `http.Server` timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`,
  `IdleTimeout`) so slow clients can't pin goroutines.

Per-request cost is essentially a `[]byte` write — no template execution, no
allocation in the hot path.

## Deployment sketch

```sh
# Build a static Linux binary on any machine
GOOS=linux GOARCH=amd64 go build -o mypage .

# Copy to your server
scp mypage user@host:/usr/local/bin/

# Run it (behind nginx/caddy or directly)
./mypage -addr=:8080
```

That's the whole deploy story — one binary, no runtime dependencies, no asset
directory to ship.
