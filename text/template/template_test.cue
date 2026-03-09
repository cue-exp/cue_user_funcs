@if(test)
@extern(inject)

package template

#NonZero: _ @inject(name="github.com/cue-exp/cue_user_funcs@test/text/template.NonZero")

trueString:  #NonZero("hello")
falseString: #NonZero("")
trueInt:     #NonZero(42)
falseInt:    #NonZero(0)
trueBool:    #NonZero(true)
falseBool:   #NonZero(false)
trueList:    #NonZero([1, 2])
falseList:   #NonZero([])
trueMap:     #NonZero({a: 1})
falseMap:    #NonZero({})
