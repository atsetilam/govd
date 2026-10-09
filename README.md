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

posts and reels (\) are tried with these methods, in order:

1. **web route definition** (\): a POST to \ for \, the same request the logged-out web page makes. the streamed reply contains the media object: progressive mp4 url (with audio), full size photos, carousel items and caption. no account or cookie is needed. the request needs the \ header and the form fields \, \, \ and \ (any random value works, sent together with the \ header).
2. GraphQL web api
3. embed page
4. igram.world (third party)

why it broke (october 2026): method 2 now answers \ to anonymous requests; method 3 no longer contains the media data in the page (only the thumbnail, \ is null); method 4 refuses the request signature (\ is gone). all three failed, so every post and reel returned an error. method 1 replaced them as the first choice; the others are kept as fallbacks but currently do not work.

limits of method 1:

* logged-out visitors get no caption for some reels, so those are delivered with an empty caption.
* a video whose file url is not exposed to logged-out visitors is reported as unavailable instead of falling back to the other methods.
* the reply format is not a public api. if instagram changes it, posts will fail again with \.

live test (needs network, skipped unless \ is set; use the shortcode of any public post or reel):

    IGID=<shortcode> go test -run TestLive -v ./internal/extractors/instagram/

it runs every method, with and without the chrome impersonation client, and logs the formats each one returns.

**stories do not work.** the story extractor only uses igram.world, which now rejects the signed request (\). there is no logged-out alternative: the story pages on instagram.com show a login wall to anonymous visitors and the route definition for them carries no media. restoring stories needs a logged-in session cookie for instagram (supplied through the extractor cookies) or a new signing scheme for igram.world. a session cookie from a secondary account can expire or be challenged by instagram without notice.

## license

this project is licensed under the [mit license](LICENSE).
