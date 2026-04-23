package diffscan

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleRemovedTests_Go(t *testing.T) {
	fd := FileDiff{
		Path: "foo_test.go",
		Removed: []string{
			"func TestBar(t *testing.T) {",
			"\trequire.Equal(t, 1, 1)",
			"}",
		},
	}
	findings := RuleRemovedTests(fd)
	require.NotEmpty(t, findings)
	require.Contains(t, findings[0].Message, "TestBar")
}

func TestRuleRemovedTests_Python(t *testing.T) {
	fd := FileDiff{
		Path:    "test_foo.py",
		Removed: []string{"def test_bar():", "    assert 1 == 1"},
	}
	require.NotEmpty(t, RuleRemovedTests(fd))
}

func TestRuleRemovedTests_JS(t *testing.T) {
	fd := FileDiff{
		Path:    "foo.test.js",
		Removed: []string{"  it('bar', () => {", "    expect(true).toBe(true);"},
	}
	require.NotEmpty(t, RuleRemovedTests(fd))
}

func TestRuleRemovedTests_NonTestChangeClean(t *testing.T) {
	fd := FileDiff{
		Path:    "foo_test.go",
		Removed: []string{"\t// comment removed"},
	}
	require.Empty(t, RuleRemovedTests(fd))
}

func TestRuleRemovedTests_NonTestFileIgnored(t *testing.T) {
	fd := FileDiff{
		Path:    "foo.go",
		Removed: []string{"func TestBar(t *testing.T) {}"},
	}
	require.Empty(t, RuleRemovedTests(fd), "rule should only fire on test files")
}

func TestRuleSkipDirective_Go(t *testing.T) {
	fd := FileDiff{
		Path:  "foo_test.go",
		Added: []string{"\tt.Skip(\"flaky\")"},
	}
	require.NotEmpty(t, RuleSkipDirective(fd))
}

func TestRuleSkipDirective_Pytest(t *testing.T) {
	fd := FileDiff{
		Path:  "test_foo.py",
		Added: []string{"@pytest.mark.skip(reason=\"flaky\")"},
	}
	require.NotEmpty(t, RuleSkipDirective(fd))
}

func TestRuleSkipDirective_JestSkip(t *testing.T) {
	fd := FileDiff{Path: "foo.test.js", Added: []string{"it.skip('bar', () => {})"}}
	require.NotEmpty(t, RuleSkipDirective(fd))
}

func TestRuleSkipDirective_JestXit(t *testing.T) {
	fd := FileDiff{Path: "foo.test.js", Added: []string{"xit('bar', () => {})"}}
	require.NotEmpty(t, RuleSkipDirective(fd))
}

func TestRuleSkipDirective_Clean(t *testing.T) {
	fd := FileDiff{Path: "foo_test.go", Added: []string{"\tt.Run(\"ok\", func(t *testing.T) {})"}}
	require.Empty(t, RuleSkipDirective(fd))
}

func TestRuleWeakenedAssertion_GoTrue(t *testing.T) {
	fd := FileDiff{
		Path:  "foo_test.go",
		Added: []string{"\trequire.True(t, true)"},
	}
	require.NotEmpty(t, RuleWeakenedAssertion(fd))
}

func TestRuleWeakenedAssertion_JestTrue(t *testing.T) {
	fd := FileDiff{Path: "foo.test.js", Added: []string{"    expect(true).toBe(true)"}}
	require.NotEmpty(t, RuleWeakenedAssertion(fd))
}

func TestRuleWeakenedAssertion_PythonTrue(t *testing.T) {
	fd := FileDiff{Path: "test_foo.py", Added: []string{"    assert True"}}
	require.NotEmpty(t, RuleWeakenedAssertion(fd))
}

func TestRuleWeakenedAssertion_Clean(t *testing.T) {
	fd := FileDiff{Path: "foo_test.go", Added: []string{"\trequire.Equal(t, 42, x)"}}
	require.Empty(t, RuleWeakenedAssertion(fd))
}

func TestRuleDeletedTestFile(t *testing.T) {
	fd := FileDiff{Path: "foo_test.go", Deleted: true}
	require.NotEmpty(t, RuleDeletedTestFile(fd))
}

func TestRuleDeletedTestFile_NonTestFileIgnored(t *testing.T) {
	fd := FileDiff{Path: "foo.go", Deleted: true}
	require.Empty(t, RuleDeletedTestFile(fd))
}

func TestRuleDeletedTestFile_NotDeletedIgnored(t *testing.T) {
	fd := FileDiff{Path: "foo_test.go", Deleted: false}
	require.Empty(t, RuleDeletedTestFile(fd))
}
