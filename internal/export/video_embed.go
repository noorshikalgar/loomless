package export

import "regexp"

// youtubeURLPattern and vimeoURLPattern recognize the handful of URL
// shapes people actually paste for these two services — youtube.com
// watch/embed links, the youtu.be short form, and vimeo.com/<id>.
// Anything else (a generic "click here to watch" link, a self-hosted
// video, a Loom link) isn't recognized and just stays a normal link —
// there's no attempt at a general "any URL might be a video" heuristic.
var (
	youtubeURLPattern = regexp.MustCompile(`^https?://(?:www\.)?(?:youtube\.com/(?:watch\?v=|embed/)|youtu\.be/)([\w-]{6,})`)
	vimeoURLPattern   = regexp.MustCompile(`^https?://(?:www\.)?vimeo\.com/(\d+)`)
)

// videoEmbedURL recognizes a YouTube or Vimeo URL and returns the
// corresponding embeddable player URL (suitable for an <iframe src>),
// or ok=false if rawURL isn't a recognized video link.
func videoEmbedURL(rawURL string) (embedURL string, ok bool) {
	if m := youtubeURLPattern.FindStringSubmatch(rawURL); m != nil {
		return "https://www.youtube.com/embed/" + m[1], true
	}
	if m := vimeoURLPattern.FindStringSubmatch(rawURL); m != nil {
		return "https://player.vimeo.com/video/" + m[1], true
	}
	return "", false
}
