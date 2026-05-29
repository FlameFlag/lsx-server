package compat

import "testing"

type packedDate struct {
	Year              int16
	Month             int16
	Day               int16
	Hour              int16
	UnusedOrDayOfWeek int16
	Minute            int16
	Second            int16
	Millisecond       int16
}

func TestComputeChecksum(t *testing.T) {
	fields := map[string]string{
		"gamestartingdate": "2",
		"revenues":         "500",
		"stands":           "1",
		"lifespan":         "3",
		"gamegoal":         "4",
		"standsassets":     "5",
		"cupssold":         "6",
		"gamemode":         "7",
		"retainedearnings": "8",
		"upgradesassets":   "9",
		"cashassets":       "10",
	}
	if got, want := ComputeChecksum(fields), int32(2485); got != want {
		t.Fatalf("ComputeChecksum() = %d, want %d", got, want)
	}
}

func TestComputePackedDateScalarUsesRecoveredFixedCalendar(t *testing.T) {
	tests := []struct {
		name string
		date packedDate
		want int32
	}{
		{
			name: "components",
			date: packedDate{Day: 2, Hour: 3, Minute: 4, Second: 5, Millisecond: 6},
			want: (((2*24+3)*60+4)*60+5)*1000 + 6,
		},
		{
			name: "one fixed month overflows int32 like x86",
			date: packedDate{Month: 1},
			want: -1702967296,
		},
		{
			name: "one fixed year",
			date: packedDate{Year: 1},
			want: 1039228928,
		},
		{
			name: "unused word ignored",
			date: packedDate{UnusedOrDayOfWeek: 99},
			want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := computePackedDateScalar(tc.date); got != tc.want {
				t.Fatalf("computePackedDateScalar() = %d, want %d", got, tc.want)
			}
		})
	}
}

func computePackedDateScalar(date packedDate) int32 {
	days := int32(date.Year)*360 + int32(date.Month)*30 + int32(date.Day)
	return (((days*24+int32(date.Hour))*60+int32(date.Minute))*60+int32(date.Second))*1000 +
		int32(date.Millisecond)
}

func TestComputeChecksumUsesSignedInt32ClientOverflow(t *testing.T) {
	fields := map[string]string{
		"gamestartingdate": "1039228928",
		"revenues":         "2000000000",
		"stands":           "-83",
		"lifespan":         "11",
		"gamegoal":         "1",
		"standsassets":     "50000",
		"cupssold":         "0",
		"gamemode":         "1",
		"retainedearnings": "-1977835773",
		"upgradesassets":   "10000",
		"cashassets":       "8291",
	}
	date := int32(1039228928)
	revenues := int32(2000000000)
	stands := int32(-83)
	lifespan := int32(11)
	gamegoal := int32(1)
	standsAssets := int32(50000)
	cupsSold := int32(0)
	gamemode := int32(1)
	retained := int32(-1977835773)
	upgrades := int32(10000)
	cash := int32(8291)
	want := date*(revenues-stands*100)*lifespan +
		gamegoal*5 -
		standsAssets -
		cupsSold +
		gamemode*7 +
		retained +
		upgrades +
		cash
	if got := ComputeChecksum(fields); got != want {
		t.Fatalf("ComputeChecksum() = %d, want %d", got, want)
	}
}
