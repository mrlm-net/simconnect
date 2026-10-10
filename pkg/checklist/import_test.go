//go:build windows

package checklist

import (
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/systems"
)

const testChecklist = "<?xml version=\"1.0\" encoding=\"Windows-1252\"?>\n" + `<SimBase.Document Type="Checklist" version="1,0"><Checklist.Checklist>
<Step ChecklistStepId="LANDING_APPROACH">
  <Page SubjectTT="TT:Landing Checklist">
    <Checkpoint ReferenceId="T_GEAR"/>
    <Block SubjectTT="TT:CONFIG">
      <Checkpoint ReferenceId="T_FLAPS"/>
      <Checkpoint ReferenceId="T_SPOILERS"/>
    </Block>
    <Checkpoint ReferenceId="T_UNKNOWN"><CheckpointDesc SubjectTT="TT:CABIN ` + "\x96" + ` CREW" ExpectationTT="TT:ADVISED"/></Checkpoint>
  </Page>
</Step>
</Checklist.Checklist></SimBase.Document>`

const testLibrary = `<?xml version="1.0" encoding="UTF-8"?>
<SimBase.Document Type="CheckpointLibrary" version="1,0"><Checklist.CheckpointLibrary>
<Checkpoint Id="T_GEAR"><CheckpointDesc SubjectTT="TT:GAME.CHECKLIST_LANDING_GEAR" ExpectationTT="TT:DOWN"/>
  <Test><TestValue><Val SimVarName="GEAR HANDLE POSITION" Units="bool"/></TestValue></Test></Checkpoint>
<Checkpoint Id="T_FLAPS"><CheckpointDesc SubjectTT="TT:Flaps" ExpectationTT="TT:FULL"/>
  <Test><TestValue><Operator OpType="EQUAL"><Val SimVarName="FLAPS HANDLE INDEX" Units="number"/><Val Value="4"/></Operator></TestValue></Test></Checkpoint>
<Checkpoint Id="T_SPOILERS"><CheckpointDesc SubjectTT="TT:Spoilers" ExpectationTT="TT:ARMED"/>
  <Sequence><Test><TestValue><Val SimVarName="SPOILERS ARMED"/></TestValue></Test><Test><TestValue><Val SimVarName="L:X"/></TestValue></Test></Sequence></Checkpoint>
</Checklist.CheckpointLibrary></SimBase.Document>`

// TestImportMSFS: a page becomes a list due at its step's stage and phase,
// blocks are flattened, localisation keys said, a Windows-1252 text read;
// simple tests of known SimVars become checks, sequences do not.
func TestImportMSFS(t *testing.T) {
	s, err := ImportMSFS("test", strings.NewReader(testChecklist), strings.NewReader(testLibrary))
	if err != nil {
		t.Fatal(err)
	}
	l, ok := s.List("landing-checklist")
	if !ok || len(s.Due("approach")) != 1 || len(s.Due("landing_approach")) != 1 {
		t.Fatalf("lists %+v", s.Lists)
	}
	var got []string
	for _, it := range l.Items {
		got = append(got, it.Challenge+"="+it.Response)
	}
	if strings.Join(got, ",") != "LANDING GEAR=DOWN,FLAPS=FULL,SPOILERS=ARMED,CABIN – CREW=ADVISED" {
		t.Errorf("items %v", got)
	}
	gear, flaps, spoilers := l.Items[0].Check, l.Items[1].Check, l.Items[2].Check
	if gear == nil || gear.Value != systems.GearDown || !*gear.Is {
		t.Errorf("gear check %+v", gear)
	}
	if flaps == nil || flaps.Value != systems.FlapsIndex || *flaps.Min != 4 || *flaps.Max != 4 {
		t.Errorf("flaps check %+v", flaps)
	}
	if spoilers != nil {
		t.Errorf("a sequence became a check: %+v", spoilers)
	}
	st := systems.State{Values: map[string]float64{systems.GearDown: 1, systems.FlapsIndex: 4}}
	if !l.Complete(st) {
		t.Error("not complete with the gear down and flaps 4")
	}
}
