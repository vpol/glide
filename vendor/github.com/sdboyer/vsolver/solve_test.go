package vsolver

import (
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
)

func TestBasicSolves(t *testing.T) {
	//solveAndBasicChecks(fixtures[len(fixtures)-1], t)
	for _, fix := range fixtures {
		solveAndBasicChecks(fix, t)
	}
}

func solveAndBasicChecks(fix fixture, t *testing.T) Result {
	sm := newdepspecSM(fix.ds, !fix.downgrade)
	l := logrus.New()

	if testing.Verbose() {
		l.Level = logrus.DebugLevel
	}

	s := NewSolver(sm, l)

	p, err := sm.GetProjectInfo(fix.ds[0].name)
	if err != nil {
		t.Error("wtf, couldn't find root project")
		t.FailNow()
	}

	var latest []ProjectName
	if fix.l == nil {
		p.Lock = dummyLock{}
		for _, ds := range fix.ds[1:] {
			latest = append(latest, ds.name.Name)
		}
	} else {
		p.Lock = fix.l
		for _, ds := range fix.ds[1:] {
			if _, has := fix.l[ds.name.Name]; !has {
				latest = append(latest, ds.name.Name)
			}
		}
	}

	result := s.Solve(p, latest)

	if fix.maxAttempts > 0 && result.Attempts > fix.maxAttempts {
		t.Errorf("(fixture: %q) Solver completed in %v attempts, but expected %v or fewer", result.Attempts, fix.maxAttempts)
	}

	if len(fix.errp) > 0 {
		if result.SolveFailure == nil {
			t.Errorf("(fixture: %q) Solver succeeded, but expected failure")
		}

		switch fail := result.SolveFailure.(type) {
		case *noVersionError:
			if fix.errp[0] != string(fail.pn) {
				t.Errorf("Expected failure on project %s, but was on project %s", fail.pn, fix.errp[0])
			}

			ep := make(map[string]struct{})
			for _, p := range fix.errp[1:] {
				ep[p] = struct{}{}
			}

			found := make(map[string]struct{})
			for _, vf := range fail.fails {
				for _, f := range getFailureCausingProjects(vf.f) {
					found[f] = struct{}{}
				}
			}

			var missing []string
			var extra []string
			for p, _ := range found {
				if _, has := ep[p]; !has {
					extra = append(extra, p)
				}
			}
			if len(extra) > 0 {
				t.Errorf("Expected solve failures due to projects %s, but solve failures also arose from %s", strings.Join(fix.errp[1:], ", "), strings.Join(extra, ", "))
			}

			for p, _ := range ep {
				if _, has := found[p]; !has {
					missing = append(missing, p)
				}
			}
			if len(missing) > 0 {
				t.Errorf("Expected solve failures due to projects %s, but %s had no failures", strings.Join(fix.errp[1:], ", "), strings.Join(missing, ", "))
			}

		default:
			// TODO round these out
			panic("unhandled solve failure type")
		}
	} else {
		if result.SolveFailure != nil {
			t.Errorf("(fixture: %q) Solver failed; error was type %T, text: %q", fix.n, result.SolveFailure, result.SolveFailure)
			return result
		}

		// Dump result projects into a map for easier interrogation
		rp := make(map[string]string)
		for _, p := range result.Projects {
			rp[string(p.Name)] = p.Version.String()
		}

		fixlen, rlen := len(fix.r), len(rp)
		if fixlen != rlen {
			// Different length, so they definitely disagree
			t.Errorf("(fixture: %q) Solver reported %v package results, result expected %v", fix.n, rlen, fixlen)
		}

		// Whether or not len is same, still have to verify that results agree
		// Walk through fixture/expected results first
		for p, v := range fix.r {
			if av, exists := rp[p]; !exists {
				t.Errorf("(fixture: %q) Project %q expected but missing from results", fix.n, p)
			} else {
				// delete result from map so we skip it on the reverse pass
				delete(rp, p)
				if v != av {
					t.Errorf("(fixture: %q) Expected version %q of project %q, but actual version was %q", fix.n, v, p, av)
				}
			}
		}

		// Now walk through remaining actual results
		for p, v := range rp {
			if fv, exists := fix.r[p]; !exists {
				t.Errorf("(fixture: %q) Unexpected project %q present in results", fix.n, p)
			} else if v != fv {
				t.Errorf("(fixture: %q) Got version %q of project %q, but expected version was %q", fix.n, v, p, fv)
			}
		}
	}

	return result
}

func getFailureCausingProjects(err error) (projs []string) {
	switch e := err.(type) {
	case *noVersionError:
		projs = append(projs, string(e.pn))
	case *disjointConstraintFailure:
		for _, f := range e.failsib {
			projs = append(projs, string(f.Depender.Name))
		}
	case *versionNotAllowedFailure:
		for _, f := range e.failparent {
			projs = append(projs, string(f.Depender.Name))
		}
	case *constraintNotAllowedFailure:
		// No sane way of knowing why the currently selected version is
		// selected, so do nothing
	default:
		panic("unknown failtype")
	}

	return
}