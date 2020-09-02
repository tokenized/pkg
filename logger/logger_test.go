package logger

import (
	"context"
	"os"
	"testing"
)

func TestLogger(test *testing.T) {
	showsystem := "showsystem"
	hidesystem := "hidesystem"
	fileName := "test.log"

	{
		os.Remove(fileName)
		logConfig := NewDevelopmentConfig()
		logConfig.EnableSubSystem(showsystem)
		logConfig.Main.AddFile(fileName)

		ctx := ContextWithLogConfig(context.Background(), logConfig)

		Log(ctx, LevelInfo, "First main entry")
		Log(ctx, LevelInfo, "First main entry with value : %d", 101)

		showCtx := ContextWithLogSubSystem(ctx, showsystem)
		Log(showCtx, LevelInfo, "First Sub entry")

		hideCtx := ContextWithLogSubSystem(ctx, hidesystem)
		Log(hideCtx, LevelInfo, "First Hidden Sub entry. You should not see this!")

		Log(ctx, LevelInfo, "Second main entry")

		ctxTrace1 := ContextWithLogTrace(ctx, "trace 1")
		Log(ctxTrace1, LevelInfo, "Entry with trace 1")

		ctxTrace2 := ContextWithLogTrace(ctx, "trace 2")
		Log(ctxTrace2, LevelInfo, "Entry with trace 2")
	}
}

func TestSubSystem(test *testing.T) {
	logConfig := NewProductionConfig()

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
	logConfig := NewProductionConfig()

	ctx := ContextWithLogConfig(context.Background(), logConfig)
	spyCtx := ContextWithLogSubSystem(ctx, "SpyNode")
	wospyCtx := ContextWithOutLogSubSystem(ctx)

	Log(ctx, LevelInfo, "Without Spynode")
	Log(spyCtx, LevelInfo, "With Spynode")
	Log(wospyCtx, LevelInfo, "Without Spynode")
}
