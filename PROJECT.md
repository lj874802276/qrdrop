# QRDrop 项目定稿文档

> **Project Name**: QRDrop
> **Tagline**: Scan a QR Code, Drop a File.
> **License**: MIT
> **Status**: MVP Development
> **Tech Stack**: Go + Native HTML/JS + SQLite
> **Business Model**: Donation-Driven

---

## 1. Project Definition

**QRDrop** is a free, open-source, ad-free, and tracker-free file delivery tool via QR code scanning. The host generates a "Dropbox" QR code; the sender scans it and uploads files anonymously—no registration, no friend requests, no app installation.

- **Primary Scenario**: Enterprise training (external lecturers/suppliers sharing files without logging into company WeChat or using USB drives).
- **Secondary Scenarios**: Offline events, client document collection, homework submission.
- **Core Principle**: All capabilities reside on the server; the client is a zero-threshold web page.

---

## 2. Product Philosophy

> See `PHILOSOPHY.md` for the full bilingual version.

1. **Tool First**: Solve real problems first. QRDrop does one thing: make file transfer as simple as scanning a QR code.
2. **Zero Friction**: Senders need no registration or app; hosts just need a browser. Technology should not be a burden.
3. **Server-Side Power**: All complexity stays on the server. Users never need to understand "services," "ports," or "deployments."
4. **Social Design**: Designed for any scenario requiring "QR code file collection," not just one company.
5. **Open & Transparent**: Fully open-source under MIT. Auditable and freely deployable.
6. **Donation-Driven**: No paywalls, no paid features. Maintenance relies on voluntary community donations.
7. **No Premature Monetization**: No business model discussions until the tool is mature.

---

## 3. Core User Flow

### Host (Receiver)
1. Open QRDrop web page (Windows PC / Mac).
2. Click "New Inbox".
3. Screen displays a **QR Code** and inbox status.
4. Lecturer / Client scans the code.
5. Web page updates in real-time with received files.
6. Click to open or download files.

### Sender (Uploader)
1. Scan QR code via WeChat / DingTalk / Feishu / Camera.
2. Mobile browser opens the upload page.
3. Select file (PPT / PDF / Image / ZIP).
4. Click Upload.
5. Done. No login required.

**Transport**: Public HTTPS. No WeChat file transfer, no same-WiFi requirement.

---

## 4. Technical Architecture

### Principles
- **Server-side logic**: All processing happens on the server.
- **Client-side**: Pure web browser. No installations.
- **Communication**: HTTPS + WebSocket (for real-time updates).
- **Security**: Random session tokens, short TTL, auto-cleanup.
- **Deployment**: Docker or single binary.

### Tech Stack (Global Standard)

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Language | **Go 1.22+** | Single binary, cross-platform, self-hosting friendly, globally familiar. |
| HTTP | Stdlib / chi / echo | Lightweight, mature ecosystem. |
| Frontend | **Native HTML/CSS/JS** | Minimalist, zero-framework, fast loading, easy to embed. |
| Styling | CSS Variables / Minimal Tailwind | Elegant, restrained, not flashy. |
| Storage | Local Disk | Sufficient for meeting room scenarios. |
| Metadata | **SQLite** | Single file, zero-maintenance, global self-hosting standard. |
| Realtime | WebSocket / Polling | Simple first, firewall-friendly. |
| QR Code | Server-side SVG/PNG generation | No frontend library dependencies. |
| Deployment | **Docker + Single Binary** | Global self-hosting standard. |
| i18n | JSON language packs + `lang` param | Chinese / English priority. |
| Large Files | TUS Protocol (Future) | International standard for resumable uploads. |

### Deployment
- **Meeting Room PC**: Double-click `qrdrop.exe` or run `./qrdrop server`.
- **Server**: `docker run -p 8080:8080 -v ./data:/app/data ghcr.io/lj874802276/qrdrop:latest`.
- **Official SaaS**: Maintained by author, free initially.

---

## 5. Language Strategy

### Phase 1 (MVP)
- Default: **Chinese**.
- Top-right toggle: `中文 | English`.
- Selection persists via `localStorage`.
- Applies to both Desktop and Mobile views.

### Phase 2 (Future)
- Auto-detect language based on IP geolocation (CN -> Chinese, Others -> English).
- Manual toggle always overrides auto-detection.

---

## 6. Business Model: Donation-Driven

| Version | Cost | Speed Limit | Features |
|---------|------|-------------|----------|
| Self-Hosted | Free | Unlimited | Full |
| Official SaaS | Free | Unlimited (initially) | Full |
| Official SaaS (Future) | Free + Donation | Optional limits for free tier | Higher quotas for donors |

- No paywalls. No forced payments.
- Donations are voluntary and symbolic.
- Financial transparency via `FINANCE.md`.

### Donation Channels
- **GitHub Sponsors** (Preferred)
- **Ko-fi / Buy Me a Coffee**
- **Afadian / Alipay / WeChat** (For Chinese users)
- Crypto (Optional)

---

## 7. UI/UX Design Principles

> Simplicity is the ultimate elegance. The UI should be "invisible"; users only see their files.

- **Vibe**: Minimalist, restrained, utilitarian.
- **Background**: White / Light Gray.
- **Lines**: Thin, light gray borders.
- **Color**: Monochrome (Black/Gray) + one brand color (e.g., QRDrop Blue).
- **Typography**: System sans-serif (Inter / PingFang / Segoe UI).
- **Components**: Slightly rounded corners, no large radii, no gradients.
- **Whitespace**: Generous padding, low information density.
- **Icons**: Linear, non-filled.
- **Animation**: Minimal, only for state transitions.

### Layout
- **Desktop**: QR Code is the visual anchor. File list below. Language toggle top-right.
- **Mobile**: Upload area in the center. Minimal text. Language toggle top-right.

---

## 8. MVP Scope

### ✅ Included
- Create Inbox (Random session)
- Generate QR Code
- Mobile upload via scan
- Real-time file list updates
- File download/open
- Chinese default + English toggle
- File type whitelist & size limit
- Auto-expiry & cleanup

### ❌ Excluded
- User system / Login
- Billing / Speed limits / Subscriptions
- Admin dashboard
- Complex configuration
- IP-based auto language detection (Phase 2)

---

## 9. Open Source & Internationalization

- **License**: MIT
- **Docs**: Bilingual (English first).
- **Repo**: English descriptions, topics, code comments, and commit messages.
- **Distribution**: Product Hunt, Reddit, Hacker News, Twitter.
- **Topics**: `qr-code`, `file-transfer`, `open-source`, `self-hosted`, `donation-driven`, `privacy-focused`.

---

## 10. Project Structure

```text
QRDrop/
├── PROJECT.md
├── README.md
├── LICENSE
├── Dockerfile
├── go.mod
├── main.go
├── web/                   # Frontend (Embedded by Go)
│   ├── index.html
│   ├── app.js
│   └── style.css
├── server/                # Backend (Go)
│   ├── handler/
│   ├── service/
│   ├── model/
│   ├── storage/
│   └── config/
└── docs/
    ├── deploy.md
    ├── usage.md
    └── api.md
```

---

## 11. Development Mantra

> **Make QRDrop run first, then make it popular.**
> **Get the QR code scanning working before changing the world.**

---

## 12. Next Steps for AI Assistant

> **Status (2026-09-24): MVP closed loop is implemented and verified end-to-end.**

1. ✅ Initialize Go module (`go mod init`) — done, using `go 1.22` + pure-Go SQLite.
2. ✅ HTTP server with embedded static file serving (`server/` + `web/` via `embed`).
3. ✅ `POST /api/session` creates a dropbox, returns token + upload QR URL.
4. ✅ `POST /api/upload` streams files to `data/<token>/` (type/size whitelist, dedup).
5. ✅ WebSocket `/ws` for real-time file list (snapshot on connect + `file`/`status` broadcasts, ping/pong keepalive) with polling fallback in the UI.
6. ✅ `Dockerfile` (pure-Go, `CGO_ENABLED=0`) and `README.md` draft.

### Remaining (optional, post-MVP)
- Bilingual `README.md` and `docs/` (deploy.md / usage.md / api.md) content.
- `go test` unit/integration suite for storage and handlers.
- TUS resumable uploads for very large files (Phase 2).
- IP-based language auto-detection (Phase 2).

---

**QRDrop runs end-to-end: create inbox → scan QR → upload on phone → host sees files in real-time → download. Tool first, donation-driven, simple and elegant.**
