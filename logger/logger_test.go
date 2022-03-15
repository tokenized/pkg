package logger

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLogger(test *testing.T) {
	showsystem := "showsystem"
	hidesystem := "hidesystem"

	{
		logConfig := NewConfig(true, false, "")
		logConfig.EnableSubSystem(showsystem)
		// fmt.Printf("Original log config : %+v\n", logConfig)

		ctx := ContextWithLogConfig(context.Background(), logConfig)

		Log(ctx, LevelInfo, "First main entry")
		Log(ctx, LevelInfo, "First main entry with value : %d", 101)

		showCtx := ContextWithLogSubSystem(ctx, showsystem)
		Log(showCtx, LevelInfo, "First Sub entry")

		hideCtx := ContextWithLogSubSystem(ctx, hidesystem)
		Log(hideCtx, LevelInfo, "First Hidden Sub entry. You should not see this!")

		// fmt.Printf("Log config after hide : %+v\n", logConfig)

		Log(ctx, LevelInfo, "Second main entry")

		ctxTrace1 := ContextWithLogTrace(ctx, "trace 1")
		Log(ctxTrace1, LevelInfo, "Entry with trace 1")
		fmt.Printf("^ should contain \"trace 1\"\n")

		ctxTrace2 := ContextWithLogTrace(ctx, "trace 2")
		Log(ctxTrace2, LevelInfo, "Entry with trace 2")
		fmt.Printf("^ should contain \"trace 2\"\n")
	}
}

func TestSubSystem(test *testing.T) {
	logConfig := NewConfig(false, false, "")

	logConfig.EnableSubSystem("SpyNode")

	ctx := ContextWithLogConfig(context.Background(), logConfig)
	log := NewLoggerObject(ctx)
	spyCtx := ContextWithLogSubSystem(ctx, "SpyNode")
	wospyCtx := ContextWithOutLogSubSystem(ctx)

	Log(ctx, LevelInfo, "Without Spynode")
	Log(spyCtx, LevelInfo, "With Spynode")
	Log(wospyCtx, LevelInfo, "Without Spynode")

	log.Printf("Print")
}

func TestDisabledSubSystem(test *testing.T) {
	logConfig := NewConfig(false, false, "")

	ctx := ContextWithLogConfig(context.Background(), logConfig)
	spyCtx := ContextWithLogSubSystem(ctx, "SpyNode")
	wospyCtx := ContextWithOutLogSubSystem(ctx)

	Log(ctx, LevelInfo, "Without Spynode")
	Log(spyCtx, LevelInfo, "With Spynode")
	Log(wospyCtx, LevelInfo, "Without Spynode")
}

func TestFields(t *testing.T) {
	ctx := ContextWithLogger(context.Background(), false, false, "")

	s := String("string", "value")
	i := Int("integer", 10)
	ui := Uint("unsigned int", uint(20))
	f := Float32("float32", 1.0)
	f64 := Float64("float64", 2.0)
	InfoWithFields(ctx, []Field{s, i, ui, f, f64}, "String, Int, Uint, Float32, Float64")

	stringWithQuotes := String("with quote", `"should escape quote`)
	stringWithBackspace := String("with backspace", "\b should escape backspace")
	InfoWithFields(ctx, []Field{stringWithQuotes, stringWithBackspace}, "String, String")

	stringWithNewLine := String("with newline", `
	should escape newline and tab`)
	InfoWithFields(ctx, []Field{stringWithNewLine}, "String")

	stringWithBackslash := String("with backslash", `\ should escape backslash`)
	InfoWithFields(ctx, []Field{stringWithBackslash}, "String")

	hex := Hex("hex", []byte{1, 2, 3})
	InfoWithFields(ctx, []Field{hex}, "Hex")

	u32s := Uint32s("uint list", []uint32{1, 2, 3})
	InfoWithFields(ctx, []Field{u32s}, "Uint32s")

	float32s := Float32s("float list", []float32{1.234, 2.948463, 3.1})
	InfoWithFields(ctx, []Field{float32s}, "Float32s")

	json := struct {
		Field1 string `json:"field_1"`
		Field2 int    `json:"field_2"`
	}{
		Field1: "value 1",
		Field2: 2,
	}
	jsonField := JSON("json_struct", &json)
	InfoWithFields(ctx, []Field{jsonField}, "JSON")
}

func Test_DuplicateFields(t *testing.T) {
	ctx := ContextWithLogger(context.Background(), false, false, "")
	ctx = ContextWithLogFields(ctx, String("duplicate", "original"))

	InfoWithFields(ctx, []Field{String("duplicate", "should not show")}, "Message")
}

func TestWaitWarning(t *testing.T) {
	ctx := ContextWithLogger(context.Background(), false, false, "")

	waitWarning := NewWaitingWarning(ctx, 500*time.Millisecond, "Print this 4 times")
	time.Sleep(2 * time.Second)
	waitWarning.Cancel()
}

func BenchmarkContextWithLogTrace(b *testing.B) {
	ctx := ContextWithLogConfig(context.Background(), NewConfig(false, false, ""))

	for i := 0; i < b.N; i++ {
		ContextWithLogTrace(ctx, "trace")
	}
}

func BenchmarkContextWithOutLogSubSystem(b *testing.B) {
	ctx := ContextWithLogConfig(context.Background(), NewConfig(false, false, ""))

	for i := 0; i < b.N; i++ {
		ContextWithOutLogSubSystem(ctx)
	}
}

func BenchmarkFileNoFields(b *testing.B) {
	os.Mkdir("./tmp", 0755)

	logFileName := "./tmp/bench_" + uuid.New().String() + ".log"
	ctx := ContextWithLogConfig(context.Background(), NewConfig(false, false, logFileName))

	for i := 0; i < b.N; i++ {
		Info(ctx, "Simple log entry %d", i)
	}

	os.Remove(logFileName)
}

func BenchmarkFileWithFields(b *testing.B) {
	os.Mkdir("./tmp", 0755)

	logFileName := "./tmp/bench_" + uuid.New().String() + ".log"
	ctx := ContextWithLogConfig(context.Background(), NewConfig(false, false, logFileName))

	for i := 0; i < b.N; i++ {
		InfoWithFields(ctx, []Field{
			String("title", "string value"),
			Int("index", i),
			Float32("float", 123.556),
		}, "Simple log entry with fields")
	}

	os.Remove(logFileName)
}

func BenchmarkDummyNoFields(b *testing.B) {
	ctx := ContextWithLogConfig(context.Background(), NewConfig(false, false, "dummy"))

	for i := 0; i < b.N; i++ {
		Info(ctx, "Simple log entry %d", i)
	}
}

func BenchmarkDummyWithFields(b *testing.B) {
	ctx := ContextWithLogConfig(context.Background(), NewConfig(false, false, "dummy"))

	for i := 0; i < b.N; i++ {
		InfoWithFields(ctx, []Field{
			String("title", "string value"),
			Int("index", i),
			Float32("float", 123.556),
		}, "Simple log entry with fields")
	}
}
