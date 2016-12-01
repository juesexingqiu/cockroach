// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// Author: Nathan VanBenschoten (nvanbenschoten@gmail.com)

package decimal

import (
	"flag"
	"fmt"
	"math"
	"testing"
	"time"

	"gopkg.in/inf.v0"

	_ "github.com/cockroachdb/cockroach/pkg/util/log" // for flags
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

var (
	flagDurationLimit = flag.Duration("limit", 0, "function execution time limit; 0 is disabled")
)

var floatDecimalEqualities = map[float64]*inf.Dec{
	-987650000: inf.NewDec(-98765, -4),
	-123.2:     inf.NewDec(-1232, 1),
	-1:         inf.NewDec(-1, 0),
	-.00000121: inf.NewDec(-121, 8),
	0:          inf.NewDec(0, 0),
	.00000121:  inf.NewDec(121, 8),
	1:          inf.NewDec(1, 0),
	123.2:      inf.NewDec(1232, 1),
	987650000:  inf.NewDec(98765, -4),
}

func TestNewDecFromFloat(t *testing.T) {
	for tf, td := range floatDecimalEqualities {
		if dec := NewDecFromFloat(tf); dec.Cmp(td) != 0 {
			t.Errorf("NewDecFromFloat(%f) expected to give %s, but got %s", tf, td, dec)
		}

		var dec inf.Dec
		if SetFromFloat(&dec, tf); dec.Cmp(td) != 0 {
			t.Errorf("SetFromFloat(%f) expected to set decimal to %s, but got %s", tf, td, dec)
		}
	}
}

func TestFloat64FromDec(t *testing.T) {
	for tf, td := range floatDecimalEqualities {
		f, err := Float64FromDec(td)
		if err != nil {
			t.Errorf("Float64FromDec(%s) expected to give %f, but returned error: %v", td, tf, err)
		}
		if f != tf {
			t.Errorf("Float64FromDec(%s) expected to give %f, but got %f", td, tf, f)
		}
	}
}

type decimalOneArgTestCase struct {
	input    string
	expected string
}

type decimalTwoArgsTestCase struct {
	input1   string
	input2   string
	expected string
}

func testDecimalSingleArgFunc(
	t *testing.T,
	f func(*inf.Dec, *inf.Dec, inf.Scale) (*inf.Dec, error),
	s inf.Scale,
	tests []decimalOneArgTestCase,
) {
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s = %s", tc.input, tc.expected), func(t *testing.T) {
			x, exp := new(inf.Dec), new(inf.Dec)
			x.SetString(tc.input)
			exp.SetString(tc.expected)

			// Test allocated return value.
			var z *inf.Dec
			var err error
			done := make(chan struct{}, 1)
			start := timeutil.Now()
			go func() {
				z, err = f(nil, x, s)
				done <- struct{}{}
			}()
			var after <-chan time.Time
			if *flagDurationLimit > 0 {
				after = time.After(*flagDurationLimit)
			}
			select {
			case <-done:
				t.Logf("execute duration: %s", timeutil.Since(start))
			case <-after:
				t.Fatalf("timedout after %s", *flagDurationLimit)
			}
			if err != nil {
				if tc.expected != err.Error() {
					t.Errorf("expected error %s, got %s", tc.expected, err)
				}
				return
			}
			if exp.Cmp(z) != 0 {
				t.Errorf("expected %s, got %s", exp, z)
			}

			// Test provided decimal mutation.
			z.SetString("0.0")
			_, _ = f(z, x, s)
			if exp.Cmp(z) != 0 {
				t.Errorf("expected %s, got %s", exp, z)
			}

			// Test same arg mutation.
			_, _ = f(x, x, s)
			if exp.Cmp(x) != 0 {
				t.Errorf("expected %s, got %s", exp, x)
			}
			x.SetString(tc.input)
		})
	}
}

func nilErrorSingle(
	f func(*inf.Dec, *inf.Dec, inf.Scale) *inf.Dec,
) func(*inf.Dec, *inf.Dec, inf.Scale) (*inf.Dec, error) {
	return func(a, b *inf.Dec, s inf.Scale) (*inf.Dec, error) {
		return f(a, b, s), nil
	}
}

func nilErrorDouble(
	f func(*inf.Dec, *inf.Dec, *inf.Dec, inf.Scale) *inf.Dec,
) func(*inf.Dec, *inf.Dec, *inf.Dec, inf.Scale) (*inf.Dec, error) {
	return func(a, b, c *inf.Dec, s inf.Scale) (*inf.Dec, error) {
		return f(a, b, c, s), nil
	}
}

func testDecimalDoubleArgFunc(
	t *testing.T,
	f func(*inf.Dec, *inf.Dec, *inf.Dec, inf.Scale) (*inf.Dec, error),
	s inf.Scale,
	tests []decimalTwoArgsTestCase,
) {
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s, %s = %s", tc.input1, tc.input2, tc.expected), func(t *testing.T) {
			x, y, exp := new(inf.Dec), new(inf.Dec), new(inf.Dec)
			if _, ok := x.SetString(tc.input1); !ok {
				t.Fatalf("could not set decimal: %s", tc.input1)
			}
			if _, ok := y.SetString(tc.input2); !ok {
				t.Fatalf("could not set decimal: %s", tc.input2)
			}

			// Test allocated return value.
			var z *inf.Dec
			var err error
			done := make(chan struct{}, 1)
			start := timeutil.Now()
			go func() {
				z, err = f(nil, x, y, s)
				done <- struct{}{}
			}()
			var after <-chan time.Time
			if *flagDurationLimit > 0 {
				after = time.After(*flagDurationLimit)
			}
			select {
			case <-done:
				t.Logf("execute duration: %s", timeutil.Since(start))
			case <-after:
				t.Fatalf("timedout after %s", *flagDurationLimit)
			}
			if err != nil {
				if tc.expected != err.Error() {
					t.Errorf("expected error %s, got %s", tc.expected, err)
				}
				return
			}
			if z == nil {
				if tc.expected != "nil" {
					t.Errorf("expected %s, got nil", tc.expected)
				}
				return
			} else if tc.expected == "nil" {
				t.Errorf("expected nil, got %s", z)
				return
			}
			if _, ok := exp.SetString(tc.expected); !ok {
				t.Errorf("could not set decimal: %s", tc.expected)
				return
			}
			if exp.Cmp(z) != 0 {
				t.Errorf("expected %s, got %s", exp, z)
			}

			// Test provided decimal mutation.
			z.SetString("0.0")
			_, _ = f(z, x, y, s)
			if exp.Cmp(z) != 0 {
				t.Errorf("expected %s, got %s", exp, z)
			}

			// Test first arg mutation.
			_, _ = f(x, x, y, s)
			if exp.Cmp(x) != 0 {
				t.Errorf("expected %s, got %s", exp, x)
			}
			x.SetString(tc.input1)

			// Test second arg mutation.
			_, _ = f(y, x, y, s)
			if exp.Cmp(y) != 0 {
				t.Errorf("expected %s, got %s", exp, y)
			}
			y.SetString(tc.input2)

			// Test both arg mutation, if possible.
			if tc.input1 == tc.input2 {
				_, _ = f(x, x, x, s)
				if exp.Cmp(x) != 0 {
					t.Errorf("expected %s, got %s", exp, x)
				}
				x.SetString(tc.input1)
			}
		})
	}
}

func TestDecimalMod(t *testing.T) {
	tests := []decimalTwoArgsTestCase{
		{"3", "2", "1"},
		{"3451204593", "2454495034", "996709559"},
		{"24544.95034", ".3451204593", "0.3283950433"},
		{".1", ".1", "0"},
		{"0", "1.001", "0"},
		{"-7.5", "2", "-1.5"},
		{"7.5", "-2", "1.5"},
		{"-7.5", "-2", "-1.5"},
	}
	modWithScale := func(z, x, y *inf.Dec, s inf.Scale) *inf.Dec {
		return Mod(z, x, y)
	}
	testDecimalDoubleArgFunc(t, nilErrorDouble(modWithScale), 0, tests)
}

func BenchmarkDecimalMod(b *testing.B) {
	rng, _ := randutil.NewPseudoRand()
	populate := func(vals []*inf.Dec) []*inf.Dec {
		for i := range vals {
			f := 0.0
			for f == 0 {
				f = rng.Float64()
			}
			vals[i] = NewDecFromFloat(f)
		}
		return vals
	}

	dividends := populate(make([]*inf.Dec, 10000))
	divisors := populate(make([]*inf.Dec, 10000))

	z := new(inf.Dec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Mod(z, dividends[i%len(dividends)], divisors[i%len(divisors)])
	}
}

func TestDecimalSqrt(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{"0.00000000001", "0.0000031622776602"},
		{"0", "0"},
		{".12345678987654321122763812", "0.3513641841117891"},
		{"4", "2"},
		{"9", "3"},
		{"100", "10"},
		{"2454495034", "49542.8605754653613946"},
		{"24544.95034", "156.6682812186308502"},
		{"1234567898765432112.2763812", "1111111110.0000000055243715"},
	}
	testDecimalSingleArgFunc(t, nilErrorSingle(Sqrt), 16, tests)
}

func TestDecimalSqrtDoubleScale(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{"234895738245234059870198705892968191574905861209834710948561902834710985610892374", "15326308696004855684990787370582512173391.71890205964489889707604945584880"},
		{"0.0000000000000000000000000000000000000000000000000000001", "0.00000000000000000000000000031623"},
		{"0.00000000001", "0.00000316227766016837933199889354"},
		{"0", "0"},
		{".12345678987654321122763812", "0.35136418411178907639479458498081"},
		{"4", "2"},
		{"9", "3"},
		{"100", "10"},
		{"2454495034", "49542.86057546536139455430949116585673"},
		{"24544.95034", "156.66828121863085021083671472749063"},
		{"1234567898765432112.2763812", "1111111110.00000000552437154552437153179097"},
	}
	testDecimalSingleArgFunc(t, nilErrorSingle(Sqrt), 32, tests)
}

func BenchmarkDecimalSqrt(b *testing.B) {
	rng, _ := randutil.NewPseudoRand()

	vals := make([]*inf.Dec, 10000)
	for i := range vals {
		vals[i] = NewDecFromFloat(math.Abs(rng.Float64()))
	}

	z := new(inf.Dec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Sqrt(z, vals[i%len(vals)], 16)
	}
}

func TestDecimalCbrt(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{"-567", "-8.2767725291433620"},
		{"-1", "-1.0"},
		{"-0.001", "-0.1"},
		{".00000001", "0.0021544346900319"},
		{".001234567898217312", "0.1072765982021206"},
		{".001", "0.1"},
		{".123", "0.4973189833268590"},
		{"0", "0"},
		{"1", "1"},
		{"2", "1.2599210498948732"},
		{"1000", "10.0"},
		{"1234567898765432112.2763812", "1072765.9821799668569064"},
	}
	testDecimalSingleArgFunc(t, nilErrorSingle(Cbrt), 16, tests)
}

func TestDecimalCbrtDoubleScale(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{"-567", "-8.27677252914336200839737332507556"},
		{"-1", "-1.0"},
		{"-0.001", "-0.1"},
		{".00000001", "0.00215443469003188372175929356652"},
		{".001234567898217312", "0.10727659820212056117037629887220"},
		{".001", "0.1"},
		{".123", "0.49731898332685904156500833828550"},
		{"0", "0"},
		{"1", "1"},
		{"2", "1.25992104989487316476721060727823"},
		{"1000", "10.0"},
		{"1234567898765432112.2763812", "1072765.98217996685690644770246374397146"},
	}
	testDecimalSingleArgFunc(t, nilErrorSingle(Cbrt), 32, tests)
}

func BenchmarkDecimalCbrt(b *testing.B) {
	rng, _ := randutil.NewPseudoRand()

	vals := make([]*inf.Dec, 10000)
	for i := range vals {
		vals[i] = NewDecFromFloat(rng.Float64())
	}

	z := new(inf.Dec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Cbrt(z, vals[i%len(vals)], 16)
	}
}

func TestDecimalLog(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{".001234567898217312", "-6.6970342501104617"},
		{".5", "-0.6931471805599453"},
		{"1", "0"},
		{"2", "0.6931471805599453"},
		{"1234.56789", "7.1184763011977896"},
		{"1234567898765432112.2763812", "41.6572527032084749"},
		{"100000000000000000000000000000000", "73.6827229758094619"},
		{"123450000000000000000000000000000", "73.8933890056125590"},
		{"1000000000000000000000000000000000", "75.9853080688035076"},
		{"10000000000000000000000000000000000000000000000", "105.9189142777261015"},
		{"1000002350000002340000000345354700000000764000009", "110.5240868137114339"},
		{"40786335175292462000000000000000000", "79.6936551719404616"},
		{"0.000000000000000000000000000000000000000000000000001", "-117.4318397426763312"},
	}
	testDecimalSingleArgFunc(t, Log, 16, tests)
}

func TestDecimalLogDoubleScale(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{".001234567898217312", "-6.69703425011046173258548487981855"},
		{".5", "-0.69314718055994530941723212145818"},
		{"1", "0"},
		{"2", "0.69314718055994530941723212145818"},
		{"1234.56789", "7.11847630119778961310397607454138"},
		{"1234567898765432112.2763812", "41.65725270320847492372271693721825"},
		{"100000000000000000000000000000000", "73.68272297580946188857572654989965"},
		{"123450000000000000000000000000000", "73.89338900561255903040963826675629"},
		{"1000000000000000000000000000000000", "75.98530806880350757259371800458402"},
		{"10000000000000000000000000000000000000000000000", "105.91891427772610146482760691548075"},
		{"1000002350000002340000000345354700000000764000009", "110.52408681371143392718404189196936"},
	}
	testDecimalSingleArgFunc(t, Log, 32, tests)
}

func TestDecimalLog10(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{".001234567898217312", "-2.9084850199400556"},
		{".001", "-3"},
		{".123", "-0.9100948885606021"},
		{"1", "0"},
		{"123", "2.0899051114393979"},
		{"1000", "3"},
		{"1234567898765432112.2763812", "18.0915149802527613"},
	}
	testDecimalSingleArgFunc(t, Log10, 16, tests)
}

func TestDecimalLog10DoubleScale(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{".001234567898217312", "-2.90848501994005559707805612700747"},
		{".001", "-3"},
		{".123", "-0.91009488856060206819556024677670"},
		{"1", "0"},
		{"123", "2.08990511143939793180443975322329"},
		{"1000", "3"},
		{"1234567898765432112.2763812", "18.09151498025276129089765759457130"},
	}
	testDecimalSingleArgFunc(t, Log10, 32, tests)
}

func TestDecimalLogN(t *testing.T) {
	tests := []decimalTwoArgsTestCase{
		{".001234567898217312", strE, "-6.6970342501104617"},
		{".001234567898217312", "10", "-2.9084850199400556"},
		{".001", "10", "-3"},
		{".123", "10", "-0.9100948885606021"},
		{"1", "10", "0"},
		{"123", "10", "2.0899051114393979"},
		{"1000", "10", "3"},
		{"1234567898765432112.2763812", strE, "41.6572527032084749"},
		{"1234567898765432112.2763812", "10", "18.0915149802527613"},
	}
	testDecimalDoubleArgFunc(t, LogN, 16, tests)
}

func TestDecimalLogNDoubleScale(t *testing.T) {
	tests := []decimalTwoArgsTestCase{
		{".001234567898217312", strE, "-6.69703425011046173258548487981855"},
		{".001234567898217312", "10", "-2.90848501994005559707805612700747"},
		{".001", "10", "-3"},
		{".123", "10", "-0.91009488856060206819556024677670"},
		{"1", "10", "0"},
		{"123", "10", "2.08990511143939793180443975322330"},
		{"1000", "10", "3"},
		{"1234567898765432112.2763812", strE, "41.65725270320847492372271693721825"},
		{"1234567898765432112.2763812", "10", "18.09151498025276129089765759457130"},
	}
	testDecimalDoubleArgFunc(t, LogN, 32, tests)
}

func BenchmarkDecimalLog(b *testing.B) {
	rng, _ := randutil.NewPseudoRand()

	vals := make([]*inf.Dec, 10000)
	for i := range vals {
		vals[i] = NewDecFromFloat(math.Abs(rng.Float64()))
	}

	z := new(inf.Dec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Log(z, vals[i%len(vals)], 16)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestDecimalExp(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{"2.1", "8.1661699125676501"},
		{"1", "2.7182818284590452"},

		{"2", "7.3890560989306502"},
		{"0.0001", "1.0001000050001667"},

		{"-7.1", "0.0008251049232659"},
		{"-0.7", "0.4965853037914095"},
		{"0.8", "2.2255409284924676"},

		{"-6.6970342501104617", "0.0012345678982173"},
		{"-0.6931471805599453", ".5"},
		{"0.6931471805599453", "2"},
		{"7.1184763011977896", "1234.5678899999999838"},

		{"41.6572527032084749", "1234567898765432082.9890763978113354"},
		{"312.345", "4463853675713824294922499817029570039069558076531218540354463291830877552758292230013819691405163925852387646041845995193867291432899941.5478729943523921"},
	}
	testDecimalSingleArgFunc(t, nilErrorSingle(Exp), 16, tests)
}

func TestDecimalExpDoubleScale(t *testing.T) {
	tests := []decimalOneArgTestCase{
		{"2.1", "8.16616991256765007344972741047863"},
		{"1", "2.71828182845904523536028747135266"},

		{"2", "7.38905609893065022723042746057501"},
		{"0.0001", "1.00010000500016667083341666805558"},

		{"-7.1", "0.00082510492326590427014622545675"},
		{"-0.7", "0.49658530379140951470480009339753"},
		{"0.8", "2.22554092849246760457953753139508"},

		{"-6.6970342501104617", "0.00123456789821731204022899358047"},
		{"-0.6931471805599453", "0.50000000000000000470861606072909"},
		{"0.6931471805599453", "1.99999999999999998116553575708365"},
		{"7.1184763011977896", "1234.56788999999998382225190704296197"},

		{"41.6572527032084749", "1234567898765432082.98907639781133543894457806069743"},
		{"312.345", "4463853675713824294922499817029570039071067102402155066185430427302882193716789316379333056308014834801825486091800565136966585643914523.82097285541721679194656689025790"},
	}
	testDecimalSingleArgFunc(t, nilErrorSingle(Exp), 32, tests)
}

func BenchmarkDecimalExp(b *testing.B) {
	rng, _ := randutil.NewPseudoRand()

	vals := make([]*inf.Dec, 100)
	for i := range vals {
		vals[i] = NewDecFromFloat(math.Abs(rng.Float64()))
		vals[i].Add(vals[i], inf.NewDec(int64(randutil.RandIntInRange(rng, 0, 100)), 0))
	}

	z := new(inf.Dec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Exp(z, vals[i%len(vals)], 16)
	}
}

func TestDecimalPow(t *testing.T) {
	tests := []decimalTwoArgsTestCase{
		{"2", "0", "1"},
		{"8.14", "1", "8.14"},
		{"-3", "2", "9"},
		{"2", "3", "8"},
		{"4", "0.5", "2"},
		{"2", "-3", "0.125"},
		{"3.14", "9.604", "59225.9915180848144580"},
		{"4.042131231", "86.9627324951673", "56558611276325345873179603915517177973179624550320948.7364709633024969"},
		{"12.56558611276325345873179603915517177973179624550320948", "1", "12.5655861127632535"},
		{"9223372036854775807123.1", "2", "85070591730234615849667701979706147052698553.61"},
		{"-9223372036854775807123.1", "2", "85070591730234615849667701979706147052698553.61"},
		{"9223372036854775807123.1", "3", "784637716923335095255678472236230098075796571287653754351907705219.391"},
		{"-9223372036854775807123.1", "3", "-784637716923335095255678472236230098075796571287653754351907705219.391"},
		{"0", "-1", "zero raised to a negative power is undefined"},
		{"0", "0", "1"},
		{"0", "2", "0"},
		{"-1", "-.1", "a negative number raised to a non-integer power yields a complex result"},
		{"0.00000458966308373723", "-31962622854859143", "argument too large"},
		{"0.00000458966", "-123415", "argument too large"},
		{"2", "-38", "argument too large"},
		{"10000000000", "500", "argument too large"},
		{"425644047350.89246", "74.4647211651881", "argument too large"},
		{"56051.85009165843", "98.23741371063426", "argument too large"},
		{"2306257620454.719", "49.18687811476825", "argument too large"},
		{"791018.4038517432", "155.94813858582634", "argument too large"},

		// Test a small number ^ large power to test integerPower slowness. The
		// first 20 digits or so of the result here are the same as postgres. That
		// is, this test case isn't being used to test accurracy, just speed. It is
		// designed to be used with the test flag `-limit 5s`.
		{
			"0.5808269481766639",
			"-5594.351782364144",
			"1012607524935722361171974431227924446765642091678504492925818123" +
				"4668717650059503717963128331510125875130132904784980198145614001" +
				"4913088390916048184487380222714505418328209917740948171831056579" +
				"0601313521353337383192484521436799422543817328359057086106028640" +
				"4489386933186347853843232710532927608240564394332451283743151619" +
				"0352441446867588739203076093935749809881771827853552232380155901" +
				"2608753698339721603332846765674837667328613288249881636061674462" +
				"2678837656484038612558150055107863478788424760744372410254298619" +
				"5250103978593756840639359757794115138467941734572962144047401830" +
				"6233664962032045359203514861575653569725518057358461399767612022" +
				"3217904443511713612591208283866337504349112014982102500043997961" +
				"6310020142702414565410865565749314934958981342824055445040488302" +
				"6268517164157769499599978010226659152098642354217038219956367209" +
				"5102059613891029316437264412894207938800254544710963713861512134" +
				"8563186991442858334128592107106629572766509341401309880002031716" +
				"4442017088903799586126688213138585003354811246368384319914878071" +
				"8825244100931081154494567995808086514445337550994395818718728815" +
				"7837862012565020114769022175395659523500388290278083079692474665" +
				"9400896667459019900920400617254102172007924917960260845468183721" +
				"7367695058226906330293588970281829627281578623735547145940282484" +
				"99120296332436525300556450916462179859643" +
				".6464995734910913",
		},
	}
	testDecimalDoubleArgFunc(t, Pow, 16, tests)
}

func TestDecimalPowDoubleScale(t *testing.T) {
	tests := []decimalTwoArgsTestCase{
		{"2", "0", "1"},
		{"8.14", "1", "8.14"},
		{"-3", "2", "9"},
		{"2", "3", "8"},
		{"4", "0.5", "2"},
		{"2", "-3", "0.125"},
		{"3.14", "9.604", "59225.99151808481445796912159493126569"},
		{"4.042131231", "86.9627324951673", "56558611276325345873179603915517177973179624550320948.73647096330249691821726648938363"},
		{"12.56558611276325345873179603915517177973179624550320948", "1", "12.56558611276325345873179603915517"},
		{"9223372036854775807123.1", "2", "85070591730234615849667701979706147052698553.61"},
		{"-9223372036854775807123.1", "2", "85070591730234615849667701979706147052698553.61"},
		{"9223372036854775807123.1", "3", "784637716923335095255678472236230098075796571287653754351907705219.391"},
		{"-9223372036854775807123.1", "3", "-784637716923335095255678472236230098075796571287653754351907705219.391"},
		{"0.00000458966308373723", "-31962622854859143", "argument too large"},
		{"0.00000458966", "-123415", "argument too large"},
		{"2", "-38", "0.000000000004"},
		{"10000000000", "500", "argument too large"},
		{"425644047350.89246", "74.4647211651881", "argument too large"},
		{"56051.85009165843", "98.23741371063426", "argument too large"},
		{"2306257620454.719", "49.18687811476825", "argument too large"},
		{"791018.4038517432", "155.94813858582634", "argument too large"},
	}
	testDecimalDoubleArgFunc(t, Pow, 32, tests)
}

func BenchmarkDecimalPow(b *testing.B) {
	rng, _ := randutil.NewPseudoRand()
	xs := make([]*inf.Dec, 100)
	ys := make([]*inf.Dec, 100)

	for i := range ys {
		ys[i] = NewDecFromFloat(math.Abs(rng.Float64()))
		ys[i].Add(ys[i], inf.NewDec(int64(randutil.RandIntInRange(rng, 0, 10)), 0))

		xs[i] = NewDecFromFloat(math.Abs(rng.Float64()))
		xs[i].Add(xs[i], inf.NewDec(int64(randutil.RandIntInRange(rng, 0, 10)), 0))
	}

	z := new(inf.Dec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Pow(z, xs[i%len(ys)], ys[i%len(ys)], 16)
		if err != nil {
			b.Fatal(err)
		}
	}
}
