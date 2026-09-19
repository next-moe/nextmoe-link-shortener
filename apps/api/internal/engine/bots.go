package engine

import (
	"regexp"
	"strings"
)

// botUA matches clients that announce themselves as automated: crawlers,
// link-preview fetchers, uptime probes and HTTP libraries. A client that
// hides behind a browser user agent is out of reach of any user-agent rule.
var botUA = regexp.MustCompile(`bot(?:[^a-z]|$)|crawl|spider|slurp|scrap|fetch|headless|phantomjs|puppeteer|playwright|selenium|lighthouse|pagespeed|preview|externalhit|externalagent|googleother|google-|mediapartners|chatgpt-user|claude-user|perplexity-user|ia_archiver|archive\.org|httrack|wget|curl/|python|aiohttp|httpx|go-http-client|okhttp|axios|node-fetch|undici|java/|apache-httpclient|libwww|lwp-|php/|ruby|perl/|postman|insomnia|whatsapp/|uptime|pingdom|statuscake|monitor`)

// IsBot reports whether a visit's user agent belongs to an automated client.
// An empty user agent counts as one: every browser sends the header.
func IsBot(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return true
	}
	// CUBOT is an Android handset brand whose model names put "bot" at a word
	// boundary ("CUBOT X30"), and its owners are people.
	ua = strings.ReplaceAll(ua, "cubot", "")
	return botUA.MatchString(ua)
}
