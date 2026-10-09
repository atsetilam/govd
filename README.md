<h1 align="center">govd</h1>
<p align="center">
  <a href="https://t.me/govd_bot">
    <img alt="govd" title="govd" src="https://i.imgur.com/Vx8Psjn.png" width="450">
  </a>
</p>

<p align="center">
  extremely lightweight downloader, inside a telegram bot.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/govdbot/govd?style=flat-square" alt="license"></a>
  <a href="https://github.com/govdbot/govd/stargazers"><img src="https://img.shields.io/github/stars/govdbot/govd?style=flat-square" alt="stars"></a>
  <a href="https://github.com/govdbot/govd/forks"><img src="https://img.shields.io/github/forks/govdbot/govd?style=flat-square" alt="forks"></a>
  <img src="https://img.shields.io/badge/docker-ready-blue?style=flat-square" alt="docker">
  <a href="https://t.me/govd_bot"><img src="https://img.shields.io/badge/telegram-@govd__bot-2CA5E0?style=flat-square&logo=telegram" alt="telegram"></a>
  <a href="https://tgbotmau.quoi.dev/?bot=govd_bot" target="_blank"><img alt="@govd_bot MAU" title="@govd_bot MAU" src="https://tgbotmau.quoi.dev/api/bot/govd_bot/mau/badge?style=flat-square"></a>
</p>

## features

* vast number of extractors supported
* extremely lightweight
    * minimal memory usage (~80MB)
    * minimal disk usage (~150MB)
* easy to deploy with docker
* highly configurable and extensible
* supports self hosted telegram bot api
* supports authentication for extractors
* available in private chats, groups and inline mode
* translation ready (i18n)

## getting started

everything you need to get started with the project can be found in the [wiki](https://github.com/govdbot/govd/wiki).


## migrating from v1

if you are migrating from govd v1 to v2, refer to the [migration tool](https://github.com/govdbot/migrate).

## community

- [official bot](https://t.me/govd_bot)
- [support chat](https://t.me/govdsupport)

## instagram extractor notes

posts and reels (`internal/extractors/instagram`) are tried with these methods, in order:

1. **web route definition** (`route.go`): a POST to `/ajax/route-definition/` for `/p/<shortcode>/`, the same request the logged-out web page makes. the streamed reply contains the media object: progressive mp4 url (with audio), full size photos, carousel items and caption. no account or cookie is needed. the request needs the `Sec-Fetch-Site: same-origin` header and the form fields `route_url`, `__a=1`, `__comet_req=7` and `lsd` (any random value works, sent together with the `X-FB-LSD` header).
2. GraphQL web api
3. embed page
4. igram.world (third party)

why it broke (october 2026): method 2 now answers `401 require_login` to anonymous requests; method 3 no longer has the media data in the page (only the thumbnail, `contextJSON` is null); method 4 refuses the request signature (`/api/convert` is gone). all three failed, so every post and reel returned an error. method 1 is now the first choice; the others stay as fallbacks but currently do not work.

limits of method 1:

* logged-out visitors get no caption for some reels, so those are delivered with an empty caption.
* a video whose file url is not exposed to logged-out visitors is reported as unavailable, without trying the other methods.
* the reply format is not a public api. if instagram changes it, posts will fail again with `media not found in route response`.

live test (needs network, skipped unless `IGID` is set; use the shortcode of any public post or reel):

    IGID=<shortcode> go test -run TestLive -v ./internal/extractors/instagram/

it runs every method, with and without the chrome impersonation client, and logs the formats each one returns.

**stories** (`story.go`) need a logged-in session. the extractor reads the story through `/api/v1/media/<story id>/info/` on instagram.com, the private api the web app uses, with the cookies from `private/cookies/instagram.txt` (netscape format; the cookies `sessionid` and `csrftoken` are required). use a throwaway account, not a personal one, and export the cookies from a logged-in browser session of that account. the file must be readable by the bot only (`chmod 600`); it is git-ignored and never committed.

the cookies are sent on that one request only. posts, reels and share links run without them (`withoutCookies`), and the file download that follows a story is made without them too. `go test ./internal/extractors/instagram/` checks this with fake cookies.

when the session expires, is logged out or is challenged by instagram, the story request answers 401/403 (or a login page) and the user gets the bot's "authentication needed" message. the extractor does not retry. to fix it, log in to the throwaway account in a browser, export the cookies again to `private/cookies/instagram.txt` and restart the bot (cookies are cached in memory until restart). a story older than 24 hours answers 404 and is reported as unavailable. if instagram asks the account for a checkpoint, solve it in the browser first.

live check (uses the real cookie file, one request; run from the repository):

    IGSTORY=https://www.instagram.com/stories/<user>/<id> go test -run TestLiveStory -v ./internal/extractors/instagram/

the old igram.world story path was removed: it now sits behind a captcha for anything that is not a browser.

## license

this project is licensed under the [mit license](LICENSE).
