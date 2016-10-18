package candyjs

import (
	"reflect"

	. "gopkg.in/check.v1"
)

func (s *CandySuite) TestIsExported(c *C) {
	c.Assert(isExported("Foo"), Equals, true)
	c.Assert(isExported("foo"), Equals, false)
}

func (s *CandySuite) TestNameToJavaScript(c *C) {
	c.Assert(nameToJavaScript("FooQux"), Equals, "fooQux")
	c.Assert(nameToJavaScript("FOOQux"), Equals, "fooQux")
	c.Assert(nameToJavaScript("Foo"), Equals, "foo")
	c.Assert(nameToJavaScript("FOO"), Equals, "foo")
}

func (s *CandySuite) TestNameToGo(c *C) {
	c.Assert(nameToGo("fooQux"), DeepEquals, []string{"FooQux", "FOOQux"})
	c.Assert(nameToGo("FooQux"), DeepEquals, []string(nil))
}

func (s *CandySuite) TestParseJsonTag(c *C) {
	c.Assert(parseJsonTag("-"), Equals, "")
	c.Assert(parseJsonTag(""), Equals, "")
	c.Assert(parseJsonTag("foo"), Equals, "foo")
	c.Assert(parseJsonTag("foo,omitempty"), Equals, "foo")
}

func (s *CandySuite) TestNameToFieldName(c *C) {
	type structWithTags struct {
		FieldA string `json:"fielda"`
		FieldB string
	}
	d := reflect.ValueOf(structWithTags{})
	c.Assert(nameToFieldName(d, "fielda"), Equals, "FieldA")
	c.Assert(nameToFieldName(d, "fieldA"), Equals, "")
	c.Assert(nameToFieldName(d, "fieldB"), Equals, "")
	c.Assert(nameToFieldName(d, "FieldB"), Equals, "FieldB")

	type structWithoutTags struct {
		FieldA string
		FIELDB string
	}
	d = reflect.ValueOf(structWithoutTags{})
	c.Assert(nameToFieldName(d, "fieldA"), Equals, "FieldA")
	c.Assert(nameToFieldName(d, "fieldB"), Equals, "FIELDB")
}
