package detector

import "testing"

func TestLabelsMatchPublicDetailsContract(t *testing.T) {
	want := []string{
		"FEMALE_GENITALIA_COVERED", "FACE_FEMALE", "BUTTOCKS_EXPOSED",
		"FEMALE_BREAST_EXPOSED", "FEMALE_GENITALIA_EXPOSED", "MALE_BREAST_EXPOSED",
		"ANUS_EXPOSED", "FEET_EXPOSED", "BELLY_COVERED", "FEET_COVERED",
		"ARMPITS_COVERED", "ARMPITS_EXPOSED", "FACE_MALE", "BELLY_EXPOSED",
		"MALE_GENITALIA_EXPOSED", "ANUS_COVERED", "FEMALE_BREAST_COVERED",
		"BUTTOCKS_COVERED",
	}

	if len(labels) != len(want) {
		t.Fatalf("labels count = %d, want %d", len(labels), len(want))
	}
	for index, label := range want {
		if labels[index] != label {
			t.Fatalf("labels[%d] = %q, want %q", index, labels[index], label)
		}
	}
}
