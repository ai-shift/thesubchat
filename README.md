# Shellshift

## Architecture
Stack: Go + HTMX (preact.js) + Turso

### Services
#### Auth

- [golang-jwt](https://github.com/golang-jwt/jwt)
- [oauth2](https://github.com/golang/oauth2) *optional*

##### Use cases

1. Login
2. Logout

#### Chat

- [ace.js](https://ace.c9.io/)

##### Use cases

1. See previous messages
2. Render markdown
3. Edit any message
4. Choose LLM
5. Write prompt message
6. Upload media (file / image)
7. C-v uploads clipboard's contents as file
8. Open in editor any added file / clipboard
9. Mention other chat

#### Branch
##### Use cases

2. Merge into main
3. Fork to the new chat

#### VCS

- [gitgraph.js](https://addshore.com/2018/03/gitgraph-js-and-codepen-io-for-git-visualization/)
- [golang git](https://github.com/go-git/go-git)

##### Use cases

1. Checkout
2. Set system prompt
3. Choose default LLM
4. Add / edit tags
5. Connect chats

#### Graph

- [https://ivis-at-bilkent.github.io/cytoscape.js-fcose/demo/demo-compound.html](cytoscape.js)

##### Use cases

1. Create new chat
2. Position chats sorted by recently used from the center
3. Connect nodes based on common tags & mentions
4. Delete chat
