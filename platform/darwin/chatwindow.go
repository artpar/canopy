package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include <stdbool.h>
#include "chatwindow.h"
*/
import "C"
import "unsafe"

// OnChatSend is called when the user presses Enter in the chat window.
var OnChatSend func(text string)

//export GoChatSendMessage
func GoChatSendMessage(text *C.char) {
	msg := C.GoString(text)
	if OnChatSend != nil {
		go OnChatSend(msg)
	}
}

// ShowChatWindow shows the chat window (creates it lazily on first call).
func ShowChatWindow() {
	C.JVShowChatWindow()
}

// HideChatWindow hides the chat window.
func HideChatWindow() {
	C.JVHideChatWindow()
}

// ChatAddUserMessage adds a user message bubble to the chat.
func ChatAddUserMessage(text string) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	C.JVChatAddUserMessage(cText)
}

// ChatAddStatusMessage adds a status/system message to the chat.
func ChatAddStatusMessage(text string) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	C.JVChatAddStatusMessage(cText)
}

// ChatSetBusy shows or hides the activity spinner in the chat window.
func ChatSetBusy(busy bool) {
	C.JVChatSetBusy(C.bool(busy))
}
