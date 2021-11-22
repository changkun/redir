# login

Lightweight SSO Login System

## Convention

1. Redirect to `login.changkun.de?redirect=origin`
2. When login success, `login.changkun.de` will redirect to origin with query parameter `token=xxx`
3. A service provider should do:
   1. Post the token to `login.changkun.de/verify` to verify if the token is a valid token or not. If the token is not valid, do nothing.
   2. If the token is valid, then authentication success, and set cookie to the user.
   3. Later request to the service provider will have the cookie with the token, each time should verify the token is valid or not internally. If valid, authorize success. If not, redirect to `login.changkun.de?redir=origin`.

## Test page

`/test`

## License

Copyright (c) 2021 Changkun Ou. All Rights Reserved.
Unauthorized using, copying, modifying and distributing,via any medium
is strictly prohibited.