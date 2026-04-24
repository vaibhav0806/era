package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasMakefileTest_PresentTarget(t *testing.T) {
	require.True(t, HasMakefileTest("testdata/makefile_with_test"))
}

func TestHasMakefileTest_NoTestTarget(t *testing.T) {
	require.False(t, HasMakefileTest("testdata/makefile_no_test"))
}

func TestHasMakefileTest_NoMakefile(t *testing.T) {
	require.False(t, HasMakefileTest("testdata/no_makefile"))
}

func TestHasMakefileTest_TestAllNotMatched(t *testing.T) {
	// Regex `^test\s*:` must NOT match `test-all:` or `test_unit:`.
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/Makefile", []byte("test-all:\n\t@echo x\n"), 0644))
	require.False(t, HasMakefileTest(tmp))
}

func TestHasMakefileTest_CommentedTargetNotMatched(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/Makefile", []byte("# test:\n\t@echo x\n"), 0644))
	require.False(t, HasMakefileTest(tmp))
}

func TestRunMakefileTest_Pass(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/Makefile",
		[]byte("test:\n\t@echo ok\n"), 0644))
	out, err := RunMakefileTest(context.Background(), tmp)
	require.NoError(t, err)
	require.Contains(t, out, "ok")
}

func TestRunMakefileTest_Fail(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/Makefile",
		[]byte("test:\n\t@echo fail && exit 1\n"), 0644))
	out, err := RunMakefileTest(context.Background(), tmp)
	require.Error(t, err)
	require.Contains(t, out, "fail")
}

func TestRunMakefileTest_Timeout(t *testing.T) {
	if os.Getenv("ERA_TEST_LONG") != "1" {
		t.Skip("set ERA_TEST_LONG=1 to run the 11-minute timeout test")
	}
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/Makefile",
		[]byte("test:\n\tsleep 660\n"), 0644))
	_, err := RunMakefileTest(context.Background(), tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "10-minute")
}

func TestPreCommitGate_NoMakefileTest_Skipped(t *testing.T) {
	tmp := t.TempDir()
	// No Makefile at all
	skipped, runErr := MaybeRunPreCommitTest(context.Background(), tmp)
	require.True(t, skipped)
	require.NoError(t, runErr)
}

func TestPreCommitGate_PassingTest_Runs(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/Makefile",
		[]byte("test:\n\t@echo ok\n"), 0644))
	skipped, runErr := MaybeRunPreCommitTest(context.Background(), tmp)
	require.False(t, skipped)
	require.NoError(t, runErr)
}

func TestPreCommitGate_FailingTest_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(tmp+"/Makefile",
		[]byte("test:\n\t@echo boom && exit 1\n"), 0644))
	skipped, runErr := MaybeRunPreCommitTest(context.Background(), tmp)
	require.False(t, skipped)
	require.Error(t, runErr)
	require.Contains(t, runErr.Error(), "tests_failed")
	require.Contains(t, runErr.Error(), "boom")
}
