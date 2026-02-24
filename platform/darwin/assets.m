#import <Cocoa/Cocoa.h>
#import <CoreText/CoreText.h>
#include "assets.h"

static NSCache *_imageCache = nil;

static NSCache* imageCache(void) {
    if (!_imageCache) {
        _imageCache = [[NSCache alloc] init];
    }
    return _imageCache;
}

void JVRegisterFont(const char* src) {
    NSString *srcStr = [NSString stringWithUTF8String:src];
    if (srcStr.length == 0) return;

    NSURL *fontURL = nil;

    // Check if it's a URL (http/https)
    if ([srcStr hasPrefix:@"http://"] || [srcStr hasPrefix:@"https://"]) {
        // Download to temp file, then register
        NSURL *remoteURL = [NSURL URLWithString:srcStr];
        if (!remoteURL) return;

        NSData *data = [NSData dataWithContentsOfURL:remoteURL];
        if (!data) return;

        NSString *tempPath = [NSTemporaryDirectory() stringByAppendingPathComponent:[remoteURL lastPathComponent]];
        [data writeToFile:tempPath atomically:YES];
        fontURL = [NSURL fileURLWithPath:tempPath];
    } else {
        // Local path — resolve relative to current directory
        NSString *path = srcStr;
        if (![path hasPrefix:@"/"]) {
            NSString *cwd = [[NSFileManager defaultManager] currentDirectoryPath];
            path = [cwd stringByAppendingPathComponent:path];
        }
        fontURL = [NSURL fileURLWithPath:path];
    }

    if (!fontURL) return;

    CFErrorRef error = NULL;
    BOOL success = CTFontManagerRegisterFontsForURL((__bridge CFURLRef)fontURL, kCTFontManagerScopeProcess, &error);
    if (!success) {
        if (error) {
            NSError *nsError = (__bridge NSError*)error;
            // Duplicate registration is not a real error
            if (nsError.code != kCTFontManagerErrorAlreadyRegistered) {
                NSLog(@"jview: failed to register font %@: %@", srcStr, nsError);
            }
            CFRelease(error);
        }
    }
}

void JVPreloadImage(const char* alias, const char* src) {
    NSString *aliasStr = [NSString stringWithUTF8String:alias];
    NSString *srcStr = [NSString stringWithUTF8String:src];
    if (aliasStr.length == 0 || srcStr.length == 0) return;

    // Check if already cached
    if ([imageCache() objectForKey:aliasStr]) return;

    NSURL *url = nil;
    if ([srcStr hasPrefix:@"http://"] || [srcStr hasPrefix:@"https://"]) {
        url = [NSURL URLWithString:srcStr];
    } else {
        NSString *path = srcStr;
        if (![path hasPrefix:@"/"]) {
            NSString *cwd = [[NSFileManager defaultManager] currentDirectoryPath];
            path = [cwd stringByAppendingPathComponent:path];
        }
        // Local file — load synchronously
        NSImage *image = [[NSImage alloc] initWithContentsOfFile:path];
        if (image) {
            [imageCache() setObject:image forKey:aliasStr];
        }
        return;
    }

    if (!url) return;

    // Async download for URLs
    NSURLSession *session = [NSURLSession sharedSession];
    NSURLSessionDataTask *task = [session dataTaskWithURL:url completionHandler:^(NSData *data, NSURLResponse *response, NSError *error) {
        if (error || !data) return;
        NSImage *image = [[NSImage alloc] initWithData:data];
        if (!image) return;
        dispatch_async(dispatch_get_main_queue(), ^{
            [imageCache() setObject:image forKey:aliasStr];
        });
    }];
    [task resume];
}

void* JVGetCachedImage(const char* alias) {
    NSString *aliasStr = [NSString stringWithUTF8String:alias];
    NSImage *image = [imageCache() objectForKey:aliasStr];
    if (image) {
        return (__bridge void*)image;
    }
    return NULL;
}
