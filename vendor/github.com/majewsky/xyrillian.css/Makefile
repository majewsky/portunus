all: $(patsubst examples/%.scss,examples/%.css,$(wildcard examples/*.scss)) examples/typography-sans-serif.html

examples/%.css: examples/%.scss *.scss
	sassc -t compressed -I . $< $@

examples/typography-sans-serif.html: examples/typography-serif.html
	sed 's/serif/sans-serif/;s/Serif/Sans-serif/' < $< > $@
