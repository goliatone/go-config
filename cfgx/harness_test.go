package cfgx

import "testing"

// testCase standardises table-driven tests across cfgx.
type testCase struct {
	name string
	run  func(t *testing.T)
}

// runTestCases executes the provided cases using t.Run, guarding against nil funcs.
func runTestCases(t *testing.T, cases []testCase) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.run == nil {
				t.Skip("no-op test case")
				return
			}
			tc.run(t)
		})
	}
}
