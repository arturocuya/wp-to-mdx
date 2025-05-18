# Wp2Mdx

Wordpress is so heavy! A single instance needs a $10/mo instance on Digital Ocean.

My dad has a ton of those, and he wants all of them running for some reason.

I created this to migrate his misc. blogs to AstroJS.

## Usage

First, make sure to create an `.env` from `.env.example` and fill the necessary env vars.

Run the project by going to the root of the project and using this command:

`go run *.go`

This will:

- Scan your WP database and download all posts and pages as HTML
- Parse the HTML pages into markdown considering
  - Images
  - Youtube video URLs
  - Gallery shortcodes
  - Video shortcodes
- Download all the static assets used across all pages

These files are meant to be used to start a new AstroJS project (or any .md based static site generator)

Once you have that running, you can also find a script in `scripts/check-urls.go` that will crawl through an AstroJS site and detect any broken links.

> Note: This project was an experiment in which I let LLMs generate most of the code with my guidance, to try "vibecoding". I didn't really liked the experience, but the code works fine.
