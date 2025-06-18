# How to run

Install required tools:
- [Go](https://go.dev/doc/install)
- [Turso](https://docs.turso.tech/quickstart)
- [geni](https://github.com/emilpriver/geni?tab=readme-ov-file#installation)

Setup DB:

[Retrieve DB credentials (only the first step)](https://docs.turso.tech/sdk/go/quickstart)

Set Env vars (can be copy pasted from `.env.example` and sourced):
```bash
export DATABASE_URL=
export DATABASE_TOKEN=
export GEMINI_API_KEY=
```

Run:

```bash
make migrate-up
```

```
go run ./cmd/server/main.go
```

# Architecture
- Stack:
    - [Go](https://go.dev/)
    - [HTMX](https://htmx.org/)
    - [Turso](https://turso.tech/)
    - [geni](https://github.com/emilpriver/geni)
    - [ace.js](https://ace.c9.io/)
    - [cytoscape.js](https://ivis-at-bilkent.github.io/cytoscape.js-fcose/demo/demo-compound.html)

## Services
### Auth

Use cases:

1. Login
2. Logout

### Chat

Use cases:

1. See previous messages
2. Render markdown
3. Edit any message
4. Choose LLM
5. Write prompt message
6. Upload media (file / image)
7. C-v uploads clipboard's contents as file
8. Open in editor any added file / clipboard
9. Mention other chat

### Branch

Use cases:

1. Merge into main
2. Fork to the new chat

### VCS

Use cases:

1. Checkout
2. Set system prompt
3. Choose default LLM
4. Add / edit tags
5. Connect chats

### Graph

Use cases:

1. Create new chat
2. Position chats sorted by recently used from the center
3. Connect nodes based on common tags & mentions
4. Delete chat
