@if(test)
@extern(inject)

package url

#Parse: _ @inject(name="net/url.Parse")

simple: #Parse("https://example.com/path?q=1#frag")
