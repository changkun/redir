# login

Lightweight SSO Login System

## Setup

Copy `.env.template` to `.env` and fill in the values:

```
LOGIN_SECRET=<jwt-signing-secret>
LOGIN_PORT=:8080
LOGIN_USERNAME=<username>
LOGIN_PASSWORD=<password>
```

## Convention

1. Redirect to `login.changkun.de?redirect=origin`
2. When login succeeds, `login.changkun.de` redirects to origin with query parameter `token=xxx` and sets an `auth` cookie scoped to `changkun.de`.
3. A service provider should:
   1. POST the token to `login.changkun.de/verify` to verify validity. The response contains `{"username": "..."}` on success.
   2. If valid, authentication succeeds. The cookie is shared across all `*.changkun.de` subdomains.
   3. Later requests can be verified by extracting the token from the `auth` cookie or `Authorization: Bearer <token>` header and calling `/verify`.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Login page (accepts `?redirect=` query param) |
| POST | `/auth` | Authenticate with `{"username", "password", "redirect"}`, returns JWT |
| POST | `/verify` | Verify JWT with `{"token"}`, returns `{"username"}` |
| GET | `/test` | Test page for verifying login status |
| GET | `/sdk.js` | JavaScript SDK for browser integration |

## Go SDK

```go
import "changkun.de/x/login"

// Verify a token
username, err := login.Verify(token)

// Handle auth from request (checks query param then cookie)
username, err := login.HandleAuth(w, r)

// Request a token with credentials
token, err := login.RequestToken(user, pass)
```

## JavaScript SDK

Include the SDK on any `*.changkun.de` page:

```html
<script src="https://login.changkun.de/sdk.js"></script>
```

API:

```js
// Check login status, returns Promise<{ok: bool, username: string}>
changkunLogin.check().then(result => {
    if (result.ok) console.log('Hello', result.username);
});

// Redirect to login page (optional: custom redirect URL)
changkunLogin.login();
changkunLogin.login('https://example.changkun.de/dashboard');

// Logout (clears cookie, optional: custom redirect URL)
changkunLogin.logout();

// Get raw auth token (from cookie or ?token= query param)
const token = changkunLogin.getToken();
```

## License

Copyright (c) 2021 Changkun Ou. All Rights Reserved.
Unauthorized using, copying, modifying and distributing, via any medium
is strictly prohibited.
