package messages

import "testing"

func TestDownloadFileNameSanitizesPathTraversal(t *testing.T) {
	msg := Message{
		Id:       "msg-1",
		FileName: "../../.ssh/authorized_keys",
	}

	got := downloadFileName(msg)
	if got != "authorized_keys" {
		t.Fatalf("expected sanitized basename, got %q", got)
	}
}

func TestDownloadFileNameSanitizesWindowsPathTraversal(t *testing.T) {
	msg := Message{
		Id:       "msg-2",
		FileName: `..\..\AppData\Roaming\startup.bat`,
	}

	got := downloadFileName(msg)
	if got != "startup.bat" {
		t.Fatalf("expected sanitized basename, got %q", got)
	}
}

func TestDownloadFileNameFallsBackForInvalidName(t *testing.T) {
	msg := Message{
		Id:       "msg-3",
		FileName: "..",
		MimeType: "image/png",
	}

	got := downloadFileName(msg)
	if got != "msg-3.png" {
		t.Fatalf("expected fallback filename, got %q", got)
	}
}
