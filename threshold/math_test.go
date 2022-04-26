package threshold

import (
	"math/big"
	"testing"
)

func TestBigMod(t *testing.T) {
	var a, b, mod, ans, mul, sub big.Int

	a.SetString("88533297610962221552004593130488936214424982924843824626945389110212938474959", 10)
	if a.String() != "88533297610962221552004593130488936214424982924843824626945389110212938474959" {
		t.Fatalf("String conversion failed : \n%s \n!= \n88533297610962221552004593130488936214424982924843824626945389110212938474959",
			a.String())
	}

	b.SetString("97255601983051289339471927126992080905008558723677714193618616387070879687969", 10)
	if b.String() != "97255601983051289339471927126992080905008558723677714193618616387070879687969" {
		t.Fatalf("String conversion failed : \n%s \n!= \n97255601983051289339471927126992080905008558723677714193618616387070879687969",
			b.String())
	}

	mod.SetString("63099239159552726317188034995074559778634677907313415971250648285041053459645", 10)
	if mod.String() != "63099239159552726317188034995074559778634677907313415971250648285041053459645" {
		t.Fatalf("String conversion failed : \n%s \n!= \n63099239159552726317188034995074559778634677907313415971250648285041053459645",
			mod.String())
	}

	ans.SetString("383b2abc72f931f450beeaa6dbd9e11a9b1b4fd630ee9d61ed67ac072757c112", 16)
	a.Mod(&a, &mod)
	if a.Cmp(&ans) != 0 {
		t.Fatalf("Mod A failed : \n%s \n!= \n%s", a.String(), ans.String())
	}

	ans.SetString("4b83d0f6b3483416312e8b0d2f7a256f345471333cbfb1c315f4aa620b3c8064", 16)
	b.Mod(&b, &mod)
	if b.Cmp(&ans) != 0 {
		t.Fatalf("Mod B failed : \n%s \n!= \n%s", a.String(), ans.String())
	}

	a.SetString("88533297610962221552004593130488936214424982924843824626945389110212938474959", 10)
	b.SetString("97255601983051289339471927126992080905008558723677714193618616387070879687969", 10)
	a.Mul(&a, &b)
	ans.SetString("a4668f1121e4835c2984003a35dd287b0dc137ed1d9cfd5d55636ca3a9f011ae830e8658dae54e78b4a754998817b82011230167fdbf7284bf2c2fd4cfab0aaf", 16)
	if a.Cmp(&ans) != 0 {
		t.Fatalf("Multiply failed : \n%s \n!= \n%s", a.String(), ans.String())
	}

	// Modulus Multiply ****************************************************************************
	a.SetString("8", 10)
	b.SetString("12", 10)
	mod.SetString("13", 10)
	ans.SetString("5", 10)
	mul = ModMultiply(a, b, mod)
	if mul.Cmp(&ans) != 0 {
		t.Fatalf("Modulus Multiply failed : \n%s \n!= \n%s", mul.String(), ans.String())
	}

	a.SetString("2", 10)
	b.SetString("7", 10)
	mod.SetString("13", 10)
	ans.SetString("1", 10)
	mul = ModMultiply(a, b, mod)
	if mul.Cmp(&ans) != 0 {
		t.Fatalf("Modulus Multiply failed : \n%s \n!= \n%s", mul.String(), ans.String())
	}

	a.SetString("12", 10)
	b.SetString("12", 10)
	mod.SetString("13", 10)
	ans.SetString("1", 10)
	mul = ModMultiply(a, b, mod)
	if mul.Cmp(&ans) != 0 {
		t.Fatalf("Modulus Multiply failed : \n%s \n!= \n%s", mul.String(), ans.String())
	}

	// Modulus Subtract ****************************************************************************
	a.SetString("12", 10)
	b.SetString("7", 10)
	mod.SetString("13", 10)
	ans.SetString("5", 10)
	sub = ModSubtract(a, b, mod)
	if sub.Cmp(&ans) != 0 {
		t.Fatalf("Modulus Subtract failed : \n%s \n!= \n%s", sub.String(), ans.String())
	}

	a.SetString("7", 10)
	b.SetString("12", 10)
	mod.SetString("13", 10)
	ans.SetString("8", 10)
	sub = ModSubtract(a, b, mod)
	if sub.Cmp(&ans) != 0 {
		t.Fatalf("Modulus Subtract failed : \n%s \n!= \n%s", sub.String(), ans.String())
	}
}
