package common

import (
	"strings"
	"testing"
)

// Cases from mermaid's common.spec.ts and DOMPurify's output in Chrome.
func TestSanitizeText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"1\n\t\tAct1: Hello 1<script src=\"http://abc.com/script1.js\"></script>1\n\t\t<b>Act2</b>:\n\t\t1<script>\n\t\t\talert('script run......');\n\t\t</script>1\n\t1",
			"1\n\t\tAct1: Hello 11\n\t\t<b>Act2</b>:\n\t\t11\n\t1"},
		{"This is a <a href=\"javascript:runHijackingScript();\">clean link</a> + <a href=\"javascript:runHijackingScript();\">clean link</a>\n  and <a href=\"javascript&colon;bypassedMining();\">me too</a>",
			"This is a <a>clean link</a> + <a>clean link</a>\n  and <a>me too</a>"},
		{`<img onerror="alert('hello');">`, "<img>"},
		{"<iframe src=\"http://abc.com/script1.js\"></iframe>\n    <iframe src=\"http://example.com/iframeexample\"></iframe>", ""},
		{"i < n ?", "i &lt; n ?"},
		{"a & b &amp; c &lt;d&gt; &copy;", "a & b &amp; c &lt;d&gt; &copy;"},
		{"<b>a</b> & b &amp; c &lt;d&gt; &copy;", "<b>a</b> &amp; b &amp; c &lt;d&gt; ©"},
		{"line<br/>two</br>three", "line<br>two<br>three"},
		{"List<String>", "List"},
		{"<b>bold", "<b>bold</b>"},
		{"<span style='color:red'>x</span>", `<span style="color:red">x</span>`},
		{"#quot;ﬂ°quot¶ß", "#quot;ﬂ°quot¶ß"},
	} {
		if got := strings.TrimSpace(SanitizeText(c.in)); got != c.want {
			t.Errorf("%q:\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestEncodeEntities(t *testing.T) {
	got := EncodeEntities("A[#9829; #quot;x#quot;]\nstyle A fill:#f9f;\nclassDef c fill:#fff,stroke:#000;")
	want := "A[ﬂ°°9829¶ß ﬂ°quot¶ßxﬂ°quot¶ß]\nstyle A fill:#f9f\nclassDef c fill:#fff,stroke:#000\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
