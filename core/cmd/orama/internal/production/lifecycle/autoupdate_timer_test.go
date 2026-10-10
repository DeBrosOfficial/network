package lifecycle

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/DeBrosOfficial/network/pkg/systemd"
)

// timerRun is a restart with a fake systemd: the order things happened in.
type timerRun struct {
	events                    []string
	timerActive               bool
	stopErr, startErr         error
	restartErr, servingErr    error
	timerUnitsSeen, servingOK bool
}

func (r *timerRun) run() error {
	units := timerUnits{
		active: func(unit string) bool {
			r.timerUnitsSeen = unit == systemd.AutoUpdateTimerName
			return r.timerActive
		},
		stop: func(unit string) error {
			r.events = append(r.events, "stop "+unit)
			return r.stopErr
		},
		start: func(unit string) error {
			r.events = append(r.events, "start "+unit)
			return r.startErr
		},
	}
	return restartWithAutoUpdatePaused(units,
		func() error { r.events = append(r.events, "restart services"); return r.restartErr },
		func() error { r.events = append(r.events, "node serving"); return r.servingErr })
}

func TestRestartWithAutoUpdatePaused_stopsTheTimerBeforeTheServicesAndRestoresItAfterTheNodeServes(t *testing.T) {
	r := &timerRun{timerActive: true}
	if err := r.run(); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop orama-autoupdate.timer", "restart services", "node serving", "start orama-autoupdate.timer"}
	if !slices.Equal(r.events, want) {
		t.Errorf("events = %v, want %v", r.events, want)
	}
	if !r.timerUnitsSeen {
		t.Error("the timer asked about is not orama-autoupdate.timer")
	}
}

func TestRestartWithAutoUpdatePaused_aTimerThatWasOffStaysOff(t *testing.T) {
	r := &timerRun{timerActive: false}
	if err := r.run(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"restart services"}; !slices.Equal(r.events, want) {
		t.Errorf("events = %v, want %v", r.events, want)
	}
}

func TestRestartWithAutoUpdatePaused_aFailedRestartStillRestoresTheTimer(t *testing.T) {
	boom := errors.New("port 53 is in use")
	r := &timerRun{timerActive: true, restartErr: boom}
	err := r.run()
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the restart's", err)
	}
	want := []string{"stop orama-autoupdate.timer", "restart services", "start orama-autoupdate.timer"}
	if !slices.Equal(r.events, want) {
		t.Errorf("events = %v, want %v (a node that is down is not waited for)", r.events, want)
	}
}

func TestRestartWithAutoUpdatePaused_aNodeThatDoesNotServeIsReportedAndTheTimerRestored(t *testing.T) {
	notServing := errors.New("rqlite has no leader")
	r := &timerRun{timerActive: true, servingErr: notServing}
	err := r.run()
	if !errors.Is(err, notServing) {
		t.Errorf("err = %v, want the node's", err)
	}
	if last := r.events[len(r.events)-1]; last != "start orama-autoupdate.timer" {
		t.Errorf("events = %v, want the timer started last", r.events)
	}
}

func TestRestartWithAutoUpdatePaused_aTimerThatCannotBeStoppedStopsTheRestart(t *testing.T) {
	r := &timerRun{timerActive: true, stopErr: errors.New("access denied")}
	err := r.run()
	if err == nil || !strings.Contains(err.Error(), "orama-autoupdate.timer") {
		t.Errorf("err = %v", err)
	}
	if want := []string{"stop orama-autoupdate.timer"}; !slices.Equal(r.events, want) {
		t.Errorf("events = %v: the services were restarted with the timer still running", r.events)
	}
}

func TestRestartWithAutoUpdatePaused_aTimerThatCannotBeStartedIsReportedWithTheCommand(t *testing.T) {
	r := &timerRun{timerActive: true, startErr: errors.New("unit masked")}
	err := r.run()
	if err == nil || !strings.Contains(err.Error(), "systemctl start orama-autoupdate.timer") {
		t.Errorf("err = %v, want the command that starts it", err)
	}
}
