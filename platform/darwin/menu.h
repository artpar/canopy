#ifndef JVIEW_MENU_H
#define JVIEW_MENU_H

#import <Cocoa/Cocoa.h>

void JVUpdateMenu(const char* surfaceID, const char* itemsJSON);
void JVPerformAction(const char* selector);

// Build an NSMenuItem from a JSON spec dictionary. Targets array retains callback objects.
NSMenuItem* JVBuildMenuItem(NSDictionary *spec, NSMutableArray *targets);

#endif
