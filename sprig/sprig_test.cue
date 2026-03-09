@if(test)
@extern(inject)

package sprig

#Untitle:       _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Untitle")
#Substr:        _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Substr")
#Nospace:       _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Nospace")
#Trunc:         _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Trunc")
#Abbrev:        _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Abbrev")
#Abbrevboth:    _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Abbrevboth")
#Initials:      _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Initials")
#Wrap:          _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Wrap")
#WrapWith:      _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.WrapWith")
#Indent:        _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Indent")
#Nindent:       _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Nindent")
#Snakecase:     _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Snakecase")
#Camelcase:     _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Camelcase")
#Kebabcase:     _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Kebabcase")
#Swapcase:      _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Swapcase")
#Plural:        _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Plural")
#SemverCompare: _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.SemverCompare")
#Semver:        _ @inject(name="github.com/cue-exp/cue_user_funcs@test/sprig.Semver")

untitle: #Untitle("Hello World")

substr1: #Substr(0, 5, "hello world")
substr2: #Substr(6, -1, "hello world")

nospace: #Nospace("hello world")

trunc1: #Trunc(5, "hello world")
trunc2: #Trunc(-5, "hello world")

abbrev1: #Abbrev(5, "hello world")
abbrev2: #Abbrev(20, "hello world")

abbrevboth: #Abbrevboth(5, 12, "1234 5678 abcdefg")

initials: #Initials("Hello World")

wrap: #Wrap(5, "this is a long string")

wrapWith: #WrapWith(5, "\t", "this is a long string")

indent: #Indent(4, "hello\nworld")

nindent: #Nindent(4, "hello\nworld")

snakecase: #Snakecase("HelloWorld")

camelcase: #Camelcase("hello_world")

kebabcase: #Kebabcase("HelloWorld")

swapcase: #Swapcase("Hello World")

plural1: #Plural("item", "items", 1)
plural2: #Plural("item", "items", 2)

semverCompare1: #SemverCompare("^1.2.0", "1.2.3")
semverCompare2: #SemverCompare("^1.2.0", "2.0.0")

semver1: #Semver("1.2.3")
semver2: #Semver("1.2.3-beta.1+c0ff33")
