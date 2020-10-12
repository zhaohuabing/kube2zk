package rpc

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRun(t *testing.T) {
	Convey("Testing Register.Run() ", t, func() {
		reg := &Register{
			Servers: []string{"127.0.0.1:2181"},
		}

		err := reg.Init()
		So(err, ShouldBeNil)

		reg2 := &Register{
			Servers: []string{"127.0.0.1:2182"},
		}

		err = reg2.Init()
		So(err, ShouldBeNil)
		reg2.conn.Close()

		reg3 := new(Register)

		err = reg3.Init()
		So(err, ShouldNotBeNil)
	})
}

func TestGetConfig(t *testing.T) {
	Convey("Testing Register.getConfig()", t, func() {
		reg := &Register{
			Servers: []string{"127.0.0.1:2181"},
		}

		err := reg.Init()
		So(err, ShouldBeNil)

		reg.getConfig("")
	})
}

func TestAdd(t *testing.T) {
	Convey("Testing Register.Add()", t, func() {
		reg := &Register{
			Servers:  []string{"127.0.0.1:2181"},
			BasePath: "/unit_test",
		}

		err := reg.Init()
		So(err, ShouldBeNil)

		err = reg.Add("foo.com", "127.0.0.1:9000")
		So(err, ShouldBeNil)

		err = reg.Add("foo.com", "127.0.0.1:9000")
		So(err, ShouldBeNil)

		err = reg.Add("foo.test.com", "127.0.0.1:9000")
		So(err, ShouldBeNil)
		err = reg.Add("foo.test.com", "127.0.0.1:9001")
		So(err, ShouldBeNil)
	})
}

func TestDelete(t *testing.T) {
	Convey("Testing Register.Delete()", t, func() {
		reg := &Register{
			Servers:  []string{"127.0.0.1:2181"},
			BasePath: "/unit_test",
		}

		err := reg.Init()
		So(err, ShouldBeNil)

		err = reg.Delete("foo.com", "127.0.0.1:9000")
		So(err, ShouldBeNil)

		err = reg.Delete("foo.com", "127.0.0.1:9000")
		So(err, ShouldBeNil)

		err = reg.Delete("foo.test.com", "127.0.0.1:9000")
		So(err, ShouldBeNil)
		err = reg.Delete("foo.test.com", "127.0.0.1:9001")
		So(err, ShouldBeNil)
		err = reg.Delete("foo.test.com2", "127.0.0.1:9001")
		So(err, ShouldBeNil)
	})
}

func TestUpdate(t *testing.T) {
	Convey("Testing Register.Update()", t, func() {
		reg := &Register{
			Servers:  []string{"127.0.0.1:2181"},
			BasePath: "/unit_test",
		}

		err := reg.Init()
		So(err, ShouldBeNil)

		err = reg.Update("foo.com", []string{"127.0.0.1:9001"})
		So(err, ShouldBeNil)
		err = reg.Update("foo.com", []string{})
		So(err, ShouldNotBeNil)

		err = reg.Update("foo.test.com", []string{"127.0.0.1:9001"})
		So(err, ShouldBeNil)
		err = reg.Delete("foo.test.com", "127.0.0.1:9001")
		So(err, ShouldBeNil)
	})
}
