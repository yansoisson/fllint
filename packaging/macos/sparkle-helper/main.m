#import <Cocoa/Cocoa.h>
#import <Sparkle/Sparkle.h>

// Minimal Cocoa app that initializes Sparkle's SPUStandardUpdaterController
// and checks for updates. Exits after the update cycle completes.
// Reads SUFeedURL and SUPublicEDKey from the parent .app bundle's Info.plist.

@interface AppDelegate : NSObject <NSApplicationDelegate, SPUUpdaterDelegate>
@property (nonatomic, strong) SPUStandardUpdaterController *updaterController;
@end

@implementation AppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    self.updaterController = [[SPUStandardUpdaterController alloc]
        initWithStartingUpdater:YES
        updaterDelegate:self
        userDriverDelegate:nil];

    [self.updaterController checkForUpdates:nil];
}

- (void)updater:(SPUUpdater *)updater
    didFinishUpdateCycleForUpdateCheck:(SPUUpdateCheck)updateCheck
    error:(NSError * _Nullable)error {
    // Give the UI a moment to dismiss, then quit
    dispatch_after(
        dispatch_time(DISPATCH_TIME_NOW, (int64_t)(1.0 * NSEC_PER_SEC)),
        dispatch_get_main_queue(), ^{
            [NSApp terminate:nil];
        });
}

@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
        AppDelegate *delegate = [[AppDelegate alloc] init];
        app.delegate = delegate;
        [app run];
    }
    return 0;
}
