// Native glue webview_go doesn't provide on macOS, compiled by cgo
// alongside desktop_darwin.go. Manual reference counting (no ARC) —
// everything created here lives for the life of the process anyway.
//
// webview_go gives us a bare NSWindow + WKWebView. A real Mac app also
// needs, and the stock window lacks:
//   - a main menu: without an Edit menu, Cmd+C / Cmd+V / Cmd+A silently
//     do nothing in text fields, and without an app menu there's no Cmd+Q;
//   - JavaScript alert() / confirm() / prompt() and file pickers:
//     WKWebView drops them unless its UI delegate implements the panel
//     methods, so the editor's "Discard all edits?" confirm would always
//     answer "no" (and see LoomlessUIDelegate on webview_go's own one);
//   - window.open(): with no delegate method to create a new view the
//     call is a no-op. We load the target in the same window instead —
//     this is a single-window app.

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static NSWindow *sheetWindow(WKWebView *wv) {
    return wv.window ?: [NSApp keyWindow];
}

// LoomlessUIDelegate replaces webview_go's own WKUIDelegate, which it
// creates autoreleased and never retains — WKWebView holds its delegate
// weakly, so that one is deallocated as soon as the window is set up
// and <input type=file> (the editor's "Insert image") stops working too.
@interface LoomlessUIDelegate : NSObject <WKUIDelegate>
@end

@implementation LoomlessUIDelegate

- (void)webView:(WKWebView *)wv runJavaScriptAlertPanelWithMessage:(NSString *)message
    initiatedByFrame:(WKFrameInfo *)frame completionHandler:(void (^)(void))done {
    NSAlert *a = [[NSAlert alloc] init];
    a.messageText = message;
    [a addButtonWithTitle:@"OK"];
    [a beginSheetModalForWindow:sheetWindow(wv) completionHandler:^(NSModalResponse r) {
        done();
    }];
}

- (void)webView:(WKWebView *)wv runJavaScriptConfirmPanelWithMessage:(NSString *)message
    initiatedByFrame:(WKFrameInfo *)frame completionHandler:(void (^)(BOOL))done {
    NSAlert *a = [[NSAlert alloc] init];
    a.messageText = message;
    [a addButtonWithTitle:@"OK"];
    [a addButtonWithTitle:@"Cancel"];
    [a beginSheetModalForWindow:sheetWindow(wv) completionHandler:^(NSModalResponse r) {
        done(r == NSAlertFirstButtonReturn);
    }];
}

- (void)webView:(WKWebView *)wv runJavaScriptTextInputPanelWithPrompt:(NSString *)prompt
    defaultText:(NSString *)defaultText initiatedByFrame:(WKFrameInfo *)frame
    completionHandler:(void (^)(NSString *))done {
    NSAlert *a = [[NSAlert alloc] init];
    a.messageText = prompt;
    [a addButtonWithTitle:@"OK"];
    [a addButtonWithTitle:@"Cancel"];
    NSTextField *field = [[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, 260, 24)];
    field.stringValue = defaultText ?: @"";
    a.accessoryView = field;
    a.window.initialFirstResponder = field;
    [a beginSheetModalForWindow:sheetWindow(wv) completionHandler:^(NSModalResponse r) {
        done(r == NSAlertFirstButtonReturn ? field.stringValue : nil);
    }];
}

- (void)webView:(WKWebView *)wv runOpenPanelWithParameters:(WKOpenPanelParameters *)params
    initiatedByFrame:(WKFrameInfo *)frame completionHandler:(void (^)(NSArray<NSURL *> *))done {
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseFiles = YES;
    panel.canChooseDirectories = params.allowsDirectories;
    panel.allowsMultipleSelection = params.allowsMultipleSelection;
    [panel beginSheetModalForWindow:sheetWindow(wv) completionHandler:^(NSModalResponse r) {
        done(r == NSModalResponseOK ? panel.URLs : nil);
    }];
}

// window.open(): load the target in this same window — it's a
// single-window app, and returning nil without loading would make the
// call silently do nothing.
- (WKWebView *)webView:(WKWebView *)wv createWebViewWithConfiguration:(WKWebViewConfiguration *)config
    forNavigationAction:(WKNavigationAction *)action windowFeatures:(WKWindowFeatures *)features {
    if (action.request.URL != nil) {
        [wv loadRequest:action.request];
    }
    return nil;
}

@end

static NSMenuItem *item(NSString *title, SEL action, NSString *key) {
    return [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key];
}

static NSMenuItem *submenu(NSString *title, NSArray<NSMenuItem *> *items) {
    NSMenuItem *top = [[NSMenuItem alloc] initWithTitle:title action:NULL keyEquivalent:@""];
    NSMenu *menu = [[NSMenu alloc] initWithTitle:title];
    for (NSMenuItem *i in items) {
        [menu addItem:i];
    }
    top.submenu = menu;
    return top;
}

static void installMainMenu(void) {
    NSMenuItem *hideOthers = item(@"Hide Others", @selector(hideOtherApplications:), @"h");
    hideOthers.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagOption;
    NSMenuItem *redo = item(@"Redo", @selector(redo:), @"z");
    redo.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagShift;

    NSMenu *bar = [[NSMenu alloc] init];
    [bar addItem:submenu(@"Loomless", @[
        item(@"About Loomless", @selector(orderFrontStandardAboutPanel:), @""),
        [NSMenuItem separatorItem],
        item(@"Hide Loomless", @selector(hide:), @"h"),
        hideOthers,
        item(@"Show All", @selector(unhideAllApplications:), @""),
        [NSMenuItem separatorItem],
        item(@"Quit Loomless", @selector(terminate:), @"q"),
    ])];
    [bar addItem:submenu(@"Edit", @[
        item(@"Undo", @selector(undo:), @"z"),
        redo,
        [NSMenuItem separatorItem],
        item(@"Cut", @selector(cut:), @"x"),
        item(@"Copy", @selector(copy:), @"c"),
        item(@"Paste", @selector(paste:), @"v"),
        item(@"Select All", @selector(selectAll:), @"a"),
    ])];
    [bar addItem:submenu(@"View", @[
        item(@"Back", @selector(goBack:), @"["),
        item(@"Forward", @selector(goForward:), @"]"),
        item(@"Reload", @selector(reload:), @"r"),
    ])];
    NSMenuItem *window = submenu(@"Window", @[
        item(@"Minimize", @selector(performMiniaturize:), @"m"),
        item(@"Zoom", @selector(performZoom:), @""),
        item(@"Close", @selector(performClose:), @"w"),
    ]);
    [bar addItem:window];
    NSApp.mainMenu = bar;
    NSApp.windowsMenu = window.submenu;
}

void loomlessInstallHooks(void *windowPtr) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    installMainMenu();

    WKWebView *wv = (WKWebView *)window.contentView;
    if (![wv isKindOfClass:[WKWebView class]]) {
        return;
    }
    wv.allowsBackForwardNavigationGestures = YES;

    // Deliberately never released: the web view only holds it weakly.
    static LoomlessUIDelegate *uiDelegate;
    uiDelegate = [[LoomlessUIDelegate alloc] init];
    wv.UIDelegate = uiDelegate;
    [window center];
    // Launched from Finder, LaunchServices brings the app forward; run
    // as `loomless app` from a terminal it would open behind it.
    [window makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
}
