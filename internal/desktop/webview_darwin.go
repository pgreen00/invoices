//go:build darwin

package desktop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

// window.confirm(). WKWebView returns false from confirm() unless the UI
// delegate implements this, which would make every Delete button a no-op.
static void invoicesConfirmPanel(id self, SEL _cmd, WKWebView *webView, NSString *message,
                                 WKFrameInfo *frame, void (^completionHandler)(BOOL)) {
	NSAlert *alert = [[NSAlert alloc] init];
	alert.messageText = message;
	[alert addButtonWithTitle:@"OK"];
	[alert addButtonWithTitle:@"Cancel"];

	NSWindow *window = webView.window;
	if (window == nil) {
		completionHandler([alert runModal] == NSAlertFirstButtonReturn);
		return;
	}
	[alert beginSheetModalForWindow:window completionHandler:^(NSModalResponse response) {
		completionHandler(response == NSAlertFirstButtonReturn);
	}];
}

// Shows the standard print panel for the page, with Save as PDF. The paper
// defaults to US Letter portrait, as print.css asks. WebKit applies the
// stylesheet's @page margins itself; the margins here mirror them as a
// fallback for pages without an @page rule.
static void invoicesRunPrint(WKWebView *webView) API_AVAILABLE(macos(11.0)) {
	NSPrintInfo *info = [[NSPrintInfo sharedPrintInfo] copy];
	info.paperSize = NSMakeSize(612, 792);
	info.orientation = NSPaperOrientationPortrait;
	info.topMargin = 36;
	info.bottomMargin = 36;
	info.leftMargin = 43.2;
	info.rightMargin = 43.2;
	info.horizontallyCentered = NO;
	info.verticallyCentered = NO;
	info.horizontalPagination = NSPrintingPaginationModeAutomatic;
	info.verticalPagination = NSPrintingPaginationModeAutomatic;
	info.dictionary[NSPrintHeaderAndFooter] = @NO;

	NSPrintOperation *op = [webView printOperationWithPrintInfo:info];
	op.showsPrintPanel = YES;
	op.showsProgressPanel = YES;
	if (webView.title.length > 0) {
		op.jobTitle = webView.title; // the default PDF file name
	}
	// WebKit's printing view starts with a zero frame and prints blank pages
	// unless it is given one.
	op.view.frame = webView.bounds;

	if (webView.window != nil) {
		[op runOperationModalForWindow:webView.window delegate:nil didRunSelector:NULL contextInfo:NULL];
	} else {
		[op runOperation];
	}
}

static void invoicesPrint(WKWebView *webView) {
	if (@available(macOS 11.0, *)) {
		invoicesRunPrint(webView);
	}
}

// window.print(). WKWebView forwards it to these private WKUIDelegate
// methods and silently does nothing if they are missing. Printing is deferred
// to the next run loop turn because the web process is blocked until the
// delegate returns, and the print operation needs it to lay out pages.
static void invoicesPrintFrame(id self, SEL _cmd, WKWebView *webView, id frame) {
	dispatch_async(dispatch_get_main_queue(), ^{
		invoicesPrint(webView);
	});
}

static void invoicesPrintFrameWithSize(id self, SEL _cmd, WKWebView *webView, id frame,
                                       CGSize pdfFirstPageSize, void (^completionHandler)(void)) {
	completionHandler();
	dispatch_async(dispatch_get_main_queue(), ^{
		invoicesPrint(webView);
	});
}

// Adds the methods above to Wails' WKUIDelegate class. WebKit checks which
// delegate methods exist when the delegate is assigned, so this must run
// before the window is created.
static bool invoicesInstallWebViewHooks(void) {
	Class cls = objc_getClass("WailsContext");
	if (cls == nil) {
		return false;
	}
	class_addMethod(cls,
		NSSelectorFromString(@"webView:runJavaScriptConfirmPanelWithMessage:initiatedByFrame:completionHandler:"),
		(IMP)invoicesConfirmPanel, "v@:@@@@?");
	class_addMethod(cls,
		NSSelectorFromString(@"_webView:printFrame:"),
		(IMP)invoicesPrintFrame, "v@:@@");
	class_addMethod(cls,
		NSSelectorFromString(@"_webView:printFrame:pdfFirstPageSize:completionHandler:"),
		(IMP)invoicesPrintFrameWithSize, "v@:@@{CGSize=dd}@?");
	return true;
}
*/
import "C"

import "log"

// InstallWebViewHooks gives the macOS webview the browser behaviours the app
// relies on: window.confirm() for delete buttons, and window.print() for the
// print view. Call it before wails.Run.
func InstallWebViewHooks() {
	if !C.invoicesInstallWebViewHooks() {
		log.Print("webview hooks not installed: Wails' WailsContext class was not found")
	}
}
