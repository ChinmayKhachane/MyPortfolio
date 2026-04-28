package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Social struct {
	Label  string
	Handle string
	URL    string
	Icon   template.HTML
}

type Experience struct {
	ID       string
	Company  string
	Role     string
	Period   string
	Location string
	LogoSrc  string // served under /static/img/...
	Summary  string
	Bullets  []string
	Tech     []string
}

type Project struct {
	Name string
	Desc string
	URL  string
	Tags []string
}

type PageData struct {
	Name        string
	Handle      string
	Tagline     string
	Location    string
	About       string
	Interests   []string
	Socials     []Social
	Experiences []Experience
	Projects    []Project
	GitHubURL   string
	Year        int
}

// cachedResponse holds a response body pre-rendered at startup, plus its
// gzip-compressed twin and an ETag. Content is fully static after boot, so
// every request is served straight from these byte slices — no template
// execution, no allocation per hit.
type cachedResponse struct {
	body     []byte
	gzipBody []byte
	etag     string
}

var (
	tmpl      *template.Template
	pageCache cachedResponse
	expCache  = make(map[string]cachedResponse)
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	tmpl = template.Must(
		template.New("").Funcs(template.FuncMap{"dict": dict}).
			ParseFS(templatesFS, "templates/*.html"),
	)

	data := buildData()

	pageCache = renderToCache("index.html", data)
	for _, e := range data.Experiences {
		expCache[e.ID] = renderToCache("experience-detail.html", e)
	}

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler(staticSub)))
	mux.HandleFunc("GET /experience/{id}", handleExperience)
	mux.HandleFunc("GET /{$}", handleIndex)

	// Timeouts keep slow / stuck clients from pinning goroutines open.
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on http://localhost%s", *addr)
	log.Fatal(srv.ListenAndServe())
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	writeCached(w, r, pageCache, "text/html; charset=utf-8", "public, max-age=300, must-revalidate")
}

func handleExperience(w http.ResponseWriter, r *http.Request) {
	c, ok := expCache[r.PathValue("id")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeCached(w, r, c, "text/html; charset=utf-8", "public, max-age=300, must-revalidate")
}

func renderToCache(name string, data any) cachedResponse {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Fatalf("startup render %s: %v", name, err)
	}
	body := buf.Bytes()

	var gz bytes.Buffer
	gzw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	_, _ = gzw.Write(body)
	_ = gzw.Close()

	sum := sha256.Sum256(body)
	return cachedResponse{
		body:     body,
		gzipBody: gz.Bytes(),
		etag:     `"` + hex.EncodeToString(sum[:8]) + `"`,
	}
}

func writeCached(w http.ResponseWriter, r *http.Request, c cachedResponse, contentType, cacheControl string) {
	h := w.Header()
	h.Set("ETag", c.etag)
	h.Set("Cache-Control", cacheControl)
	h.Set("Vary", "Accept-Encoding")

	if match := r.Header.Get("If-None-Match"); match != "" && match == c.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h.Set("Content-Type", contentType)
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		h.Set("Content-Length", strconv.Itoa(len(c.gzipBody)))
		_, _ = w.Write(c.gzipBody)
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(c.body)))
	_, _ = w.Write(c.body)
}

func staticHandler(sub fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Caching disabled while iterating on logos / static assets so a
		// rebuild always serves fresh bytes. Switch back to
		// "public, max-age=86400" before deploying.
		w.Header().Set("Cache-Control", "no-store")
		fileServer.ServeHTTP(w, r)
	})
}

// =============================================================================
// PORTFOLIO CONTENT — edit the values below to personalise the site.
// Everything visible on the page comes from this one function.
// =============================================================================

func buildData() PageData {
	return PageData{
		// EDIT — your full name; appears in the hero and footer.
		Name: "Chinmay Khachane",

		// EDIT — short handle shown in the nav as "~/handle_". No spaces.
		Handle: "chinmay",

		// EDIT — one-line tagline rendered under the hero name.
		Tagline: "software engineer",

		// EDIT — city/region; shown in the about sidebar prose.
		Location: "earth (San Jose)",

		// EDIT — the main about-me paragraph. Plain text, no HTML.
		About: "I build backend systems and developer tooling — message queues, " +
			"feed pipelines, MCP servers, the occasional Redis clone. I enjoy " +
			"taking apart the abstractions I rely on and rebuilding smaller " +
			"versions to see how they tick.",

		// EDIT — interest tags rendered as #-prefixed chips under the hero.
		Interests: []string{
			"Distributed Systems", "Go", "Databases", "Python",
			"Data Processing", "coffee", "Deadlock (The Game)",
		},

		// EDIT — your social links. Order in the slice = order on the page.
		// Replace URL and Handle text. Icons (LinkedIn / Instagram / Mail) are
		// SVG constants at the bottom of this file — to add a service like
		// Twitter/X, Bluesky, or Mastodon, define a new icon there and add an
		// entry here.
		Socials: []Social{
			{
				Label:  "linkedin",
				Handle: "in/chinmaykhachane",                                      // EDIT — cosmetic handle text shown next to the icon.
				URL:    "https://www.linkedin.com/in/chinmay-khachane-0936a7166/", // EDIT — destination URL.
				Icon:   iconLinkedIn,
			},
			{
				Label:  "instagram",
				Handle: "@chinmay_kha",                           // EDIT
				URL:    "https://www.instagram.com/chinmay_kha/", // EDIT
				Icon:   iconInstagram,
			},
			{
				Label:  "email",
				Handle: "chinmaykhachane@gmail.com",        // EDIT
				URL:    "mailto:chinmaykhachane@gmail.com", // EDIT
				Icon:   iconMail,
			},
		},

		// EDIT — work experience entries. Each becomes a clickable tab on the
		// left of the experience section; clicking it HTMX-swaps the detail
		// card on the right. To use a real company logo, drop a file into
		// `static/img/` and point `LogoSrc` at "/static/img/yourfile.svg".
		// `ID` MUST be a unique URL-safe slug — it is what HTMX requests as
		// `/experience/{id}`. Order in the slice = order of the tabs.
		Experiences: []Experience{
			{
				ID:       "wex",
				Company:  "Wex Inc.",
				Role:     "Software Engineer Intern",
				Period:   "May 2025 — August 2025",
				Location: "Remote",
				LogoSrc:  "/static/img/wex.jpeg",
				Summary:  "Built data-lakehouse maintenance tooling and the company's internal Spark framework for ETL/ELT pipelines.",
				Bullets: []string{
					"Led initiative to develop a critical automated table maintenance service for Apache Iceberg tables using Spark and Airflow, preventing table failures and data loss while processing hundreds of tables within the company's data lakehouse — reducing storage footprint by 27% and query latency by 56%.",
					"Owned deployment lifecycle across AWS and Azure, authoring Terraform modules and GitHub Actions CI/CD pipelines with gated unit and integration testing across dev, staging, and production.",
					"Engineered DEHub, the company's internal Spark framework for ETL/ELT pipeline development, providing reusable components for data extraction, transformation, and loading to streamline pipeline workloads and enable Data Lake adoption across all lines of business.",
					"Drove adoption of the Iceberg maintenance service across 3 engineering teams, presenting technical demos to management and iterating on features based on user feedback to achieve company-wide rollout.",
				},
				Tech: []string{"Spark", "Airflow", "Iceberg", "AWS", "Azure", "Terraform", "GitHub Actions"},
			},
			{
				ID:       "aubot",
				Company:  "Aubot",
				Role:     "Software Engineer Intern",
				Period:   "February 2025 — May 2025",
				Location: "Remote",
				LogoSrc:  "/static/img/aubot.png",
				Summary:  "Hardened the API surface, authored coding curriculum at scale, and rebuilt internal documentation.",
				Bullets: []string{
					"Developed a comprehensive Python unit testing suite covering 60+ API endpoints, reducing production errors by 40% and eliminating critical security vulnerabilities.",
					"Designed and developed 4,000+ coding exercises for Advanced Python, SQL, and C++ with automated test cases and tiered difficulty progressions, adopted across 500+ active learners.",
					"Led cross-functional curriculum reviews using data-driven performance metrics, driving iterative updates that improved average student assessment scores by 20%.",
					"Rebuilt centralized documentation repository, standardizing content structure and improving searchability to serve as a single source of truth, reducing onboarding time by 35%.",
				},
				Tech: []string{"Python", "SQL", "JavaScript", "Testing"},
			},
			{
				ID:       "sjsu-research",
				Company:  "SJSU Research Foundation",
				Role:     "Machine Learning Research Assistant",
				Period:   "August 2022 — December 2024",
				Location: "San Jose, CA",
				LogoSrc:  "/static/img/sjsu.png",
				Summary:  "Built a code-matching ML system that personalized feedback for student programmers.",
				Bullets: []string{
					"Developed a predictive code-matching model leveraging student submission history and feedback patterns to forecast code improvement trajectories, achieving a 43% average improvement in student assignment scores.",
					"Conducted literature review and experimentation on code similarity techniques including AST-based comparison, cosine similarity, and embedding-based approaches, translating findings into measurable enhancements to the matching model.",
					"Built a Python evaluation framework with precision and recall metrics to measure suggestion effectiveness, enabling data-driven iteration on the feedback system's recommendation accuracy.",
					"Engineered feature extraction pipelines from raw submission data to generate structured inputs for the predictive model, delivering targeted, personalized insights to students.",
				},
				Tech: []string{"Python", "ML", "AST", "Embeddings", "FAISS"},
			},
		},

		// EDIT — featured project cards (2–6 looks best). Anything beyond
		// that can live behind the "git clone --everything-else" CTA, which
		// just links to GitHubURL below.
		Projects: []Project{
			{
				Name: "MessageQueue",
				Desc: "A Single Producer Multiple Consumer queue written in Go",
				URL:  "https://github.com/ChinmayKhachane/Message-Queue", // EDIT — full repo URL
				Tags: []string{"Go"},
			},
			{
				Name: "Folio",
				Desc: "A Multi-Brokerage Portfolio Aggregator for Fidelity, Robinhood & Wellstrade",
				URL:  "https://github.com/ChinmayKhachane/Folio", // EDIT
				Tags: []string{"Python", "React"},
			},
			{
				Name: "CommandClassifier",
				Desc: "A discord bot that analuyzes messages and provides likely commands",
				URL:  "https://github.com/ChinmayKhachane/CommandClassifier", // EDIT
				Tags: []string{"Python", "Discord API"},
			},
			{
				Name: "secEdgarMCP",
				Desc: "MCP server exposing SEC EDGAR filings to LLM agents.",
				URL:  "https://github.com/ChinmayKhachane/SecEdgarMCP", // EDIT
				Tags: []string{"MCP", "Go"},
			},
		},

		// EDIT — your GitHub profile URL. Powers the nav button and the
		// "git clone --everything-else" CTA at the bottom of the projects section.
		GitHubURL: "https://github.com/",

		Year: time.Now().Year(),
	}
}

// dict lets templates pass keyword args to nested partials: {{template "x" (dict "K" v)}}
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict expects an even number of args, got %d", len(pairs))
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key #%d is not a string", i/2)
		}
		m[k] = pairs[i+1]
	}
	return m, nil
}

// SVG icons inlined so the binary is self-contained and there is no extra HTTP round-trip.
// EDIT — to add a new social (e.g. Twitter/X, Bluesky, Mastodon), define another
// `template.HTML` constant here with the icon's SVG and reference it in `Socials`
// above. Use `currentColor` for fills/strokes so the icon picks up its parent's colour.
const (
	iconLinkedIn  template.HTML = `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M4.98 3.5C4.98 4.88 3.87 6 2.5 6S0 4.88 0 3.5 1.12 1 2.5 1s2.48 1.12 2.48 2.5zM.22 8h4.56v14H.22V8zm7.5 0h4.37v1.92h.06c.61-1.15 2.1-2.36 4.32-2.36 4.62 0 5.47 3.04 5.47 6.99V22h-4.56v-6.2c0-1.48-.03-3.38-2.06-3.38s-2.38 1.61-2.38 3.27V22H7.72V8z"/></svg>`
	iconInstagram template.HTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="5"/><circle cx="12" cy="12" r="4"/><circle cx="17.5" cy="6.5" r="1" fill="currentColor"/></svg>`
	iconMail      template.HTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/></svg>`
)
