package secret

import (
	"testing"
)

func BenchmarkHashAESKey(b *testing.B) {
	// Results on an Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz with 4 threads:
	// BenchmarkHashAESKey-8    18  301472836 ns/op (~300 ms/op)

	pass := make([]byte, 16)
	salt := make([]byte, saltSize)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hashAESKey(pass, salt)
		}
	})
}

func TestEncryptedFileDriver(t *testing.T) {
	secretDir := t.TempDir()

	const password = "correcthorsebatterystaple"

	doEncryptedFileTest(t, func() *EncryptedFile {
		return EncryptedFileDriver(password, secretDir)
	})
}

func TestSaltedFileDriver(t *testing.T) {
	secretDir := t.TempDir()

	doEncryptedFileTest(t, func() *EncryptedFile {
		return SaltedFileDriver(secretDir)
	})
}

func doEncryptedFileTest(t *testing.T, encFunc func() *EncryptedFile) {
	values := map[string]string{
		"hello": "世界",
		"zero":  string(make([]byte, 1024)),
	}

	enc := encFunc()

	for k, v := range values {
		if err := enc.Set(k, []byte(v)); err != nil {
			t.Fatalf("failed to set key %q: %v", k, err)
		}
	}

	for k, v := range values {
		b, err := enc.Get(k)
		if err != nil {
			t.Fatalf("failed to get key %q: %v", k, err)
		}
		if v != string(b) {
			t.Fatalf("value mismatch for key %q:\n-> %v\n<- %v", k, v, b)
		}
	}

	t.Run("new", func(t *testing.T) {
		newEnc := encFunc()

		for k, v := range values {
			b, err := newEnc.Get(k)
			if err != nil {
				t.Fatalf("failed to get key %q: %v", k, err)
			}
			if v != string(b) {
				t.Fatalf("value mismatch for key %q:\n-> %v\n<- %v", k, v, b)
			}
		}
	})
}
