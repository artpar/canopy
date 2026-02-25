#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "outlineview.h"

extern void GoCallbackInvoke(uint64_t callbackID, const char* data);

static const void *kOutlineDataSourceKey = &kOutlineDataSourceKey;
static const void *kOutlineDelegateKey = &kOutlineDelegateKey;
static const void *kOutlineInnerKey = &kOutlineInnerKey;

// --- OutlineItem: wrapper for a tree node ---
@interface JVOutlineItem : NSObject
@property (nonatomic, strong) NSString *itemID;
@property (nonatomic, strong) NSString *label;
@property (nonatomic, strong) NSString *iconName;
@property (nonatomic, strong) NSMutableArray<JVOutlineItem*> *children;
@end

@implementation JVOutlineItem
- (instancetype)init {
    self = [super init];
    if (self) {
        _children = [NSMutableArray array];
    }
    return self;
}
@end

// --- Data Source ---
@interface JVOutlineDataSource : NSObject <NSOutlineViewDataSource>
@property (nonatomic, strong) NSMutableArray<JVOutlineItem*> *rootItems;
@property (nonatomic, strong) NSString *labelKey;
@property (nonatomic, strong) NSString *childrenKey;
@property (nonatomic, strong) NSString *iconKey;
@property (nonatomic, strong) NSString *idKey;
@end

@implementation JVOutlineDataSource

- (instancetype)initWithLabelKey:(NSString*)lk childrenKey:(NSString*)ck iconKey:(NSString*)ik idKey:(NSString*)idk {
    self = [super init];
    if (self) {
        _rootItems = [NSMutableArray array];
        _labelKey = lk;
        _childrenKey = ck;
        _iconKey = ik;
        _idKey = idk;
    }
    return self;
}

- (void)parseJSON:(NSString*)jsonString {
    [self.rootItems removeAllObjects];
    if (!jsonString || jsonString.length == 0) return;

    NSData *data = [jsonString dataUsingEncoding:NSUTF8StringEncoding];
    NSError *error = nil;
    id parsed = [NSJSONSerialization JSONObjectWithData:data options:0 error:&error];
    if (error || ![parsed isKindOfClass:[NSArray class]]) return;

    for (NSDictionary *dict in (NSArray*)parsed) {
        JVOutlineItem *item = [self itemFromDict:dict];
        if (item) [self.rootItems addObject:item];
    }
}

- (JVOutlineItem*)itemFromDict:(NSDictionary*)dict {
    if (![dict isKindOfClass:[NSDictionary class]]) return nil;

    JVOutlineItem *item = [[JVOutlineItem alloc] init];
    item.itemID = [dict[self.idKey] description] ?: @"";
    item.label = [dict[self.labelKey] description] ?: @"";

    if (self.iconKey.length > 0) {
        id iconVal = dict[self.iconKey];
        if (iconVal) item.iconName = [iconVal description];
    }

    id childrenVal = dict[self.childrenKey];
    if ([childrenVal isKindOfClass:[NSArray class]]) {
        for (NSDictionary *childDict in (NSArray*)childrenVal) {
            JVOutlineItem *child = [self itemFromDict:childDict];
            if (child) [item.children addObject:child];
        }
    }
    return item;
}

- (JVOutlineItem*)findItemByID:(NSString*)targetID inItems:(NSArray<JVOutlineItem*>*)items {
    for (JVOutlineItem *item in items) {
        if ([item.itemID isEqualToString:targetID]) return item;
        JVOutlineItem *found = [self findItemByID:targetID inItems:item.children];
        if (found) return found;
    }
    return nil;
}

#pragma mark - NSOutlineViewDataSource

- (NSInteger)outlineView:(NSOutlineView *)outlineView numberOfChildrenOfItem:(id)item {
    if (item == nil) return self.rootItems.count;
    return ((JVOutlineItem*)item).children.count;
}

- (id)outlineView:(NSOutlineView *)outlineView child:(NSInteger)index ofItem:(id)item {
    if (item == nil) return self.rootItems[index];
    return ((JVOutlineItem*)item).children[index];
}

- (BOOL)outlineView:(NSOutlineView *)outlineView isItemExpandable:(id)item {
    return ((JVOutlineItem*)item).children.count > 0;
}

@end

// --- Delegate ---
@interface JVOutlineViewDelegate : NSObject <NSOutlineViewDelegate>
@property (nonatomic, assign) uint64_t callbackID;
@property (nonatomic, assign) BOOL suppressSelection;
@end

@implementation JVOutlineViewDelegate

- (NSView *)outlineView:(NSOutlineView *)outlineView viewForTableColumn:(NSTableColumn *)tableColumn item:(id)item {
    JVOutlineItem *outlineItem = (JVOutlineItem*)item;

    NSTableCellView *cellView = [outlineView makeViewWithIdentifier:@"JVOutlineCell" owner:self];
    if (!cellView) {
        cellView = [[NSTableCellView alloc] init];
        cellView.identifier = @"JVOutlineCell";

        NSImageView *imageView = [[NSImageView alloc] init];
        imageView.translatesAutoresizingMaskIntoConstraints = NO;
        [imageView setContentHuggingPriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationHorizontal];

        NSTextField *textField = [NSTextField labelWithString:@""];
        textField.translatesAutoresizingMaskIntoConstraints = NO;
        textField.lineBreakMode = NSLineBreakByTruncatingTail;

        [cellView addSubview:imageView];
        [cellView addSubview:textField];
        cellView.imageView = imageView;
        cellView.textField = textField;

        [NSLayoutConstraint activateConstraints:@[
            [imageView.leadingAnchor constraintEqualToAnchor:cellView.leadingAnchor constant:2],
            [imageView.centerYAnchor constraintEqualToAnchor:cellView.centerYAnchor],
            [imageView.widthAnchor constraintEqualToConstant:16],
            [imageView.heightAnchor constraintEqualToConstant:16],
            [textField.leadingAnchor constraintEqualToAnchor:imageView.trailingAnchor constant:4],
            [textField.trailingAnchor constraintEqualToAnchor:cellView.trailingAnchor constant:-2],
            [textField.centerYAnchor constraintEqualToAnchor:cellView.centerYAnchor],
        ]];
    }

    cellView.textField.stringValue = outlineItem.label ?: @"";

    if (outlineItem.iconName.length > 0) {
        NSImage *img = [NSImage imageWithSystemSymbolName:outlineItem.iconName accessibilityDescription:outlineItem.label];
        cellView.imageView.image = img;
        cellView.imageView.hidden = NO;
    } else {
        cellView.imageView.image = nil;
        cellView.imageView.hidden = YES;
    }

    return cellView;
}

- (CGFloat)outlineView:(NSOutlineView *)outlineView heightOfRowByItem:(id)item {
    return 24;
}

- (void)outlineViewSelectionDidChange:(NSNotification *)notification {
    if (self.suppressSelection) return;

    NSOutlineView *outlineView = notification.object;
    NSInteger row = outlineView.selectedRow;
    if (row < 0) return;

    JVOutlineItem *item = [outlineView itemAtRow:row];
    if (item && self.callbackID != 0) {
        const char *val = [item.itemID UTF8String];
        GoCallbackInvoke(self.callbackID, val);
    }
}

@end

// --- C API ---

void* JVCreateOutlineView(const char* dataJSON, const char* labelKey,
                           const char* childrenKey, const char* iconKey,
                           const char* idKey, const char* selectedID,
                           uint64_t callbackID) {
    NSString *lk = [NSString stringWithUTF8String:labelKey];
    NSString *ck = [NSString stringWithUTF8String:childrenKey];
    NSString *ik = [NSString stringWithUTF8String:iconKey];
    NSString *idk = [NSString stringWithUTF8String:idKey];

    // Create data source
    JVOutlineDataSource *dataSource = [[JVOutlineDataSource alloc] initWithLabelKey:lk childrenKey:ck iconKey:ik idKey:idk];
    [dataSource parseJSON:[NSString stringWithUTF8String:dataJSON]];

    // Create outline view
    NSOutlineView *outlineView = [[NSOutlineView alloc] init];
    outlineView.headerView = nil;
    outlineView.indentationPerLevel = 16;
    outlineView.rowSizeStyle = NSTableViewRowSizeStyleSmall;
    if (@available(macOS 11.0, *)) {
        outlineView.style = NSTableViewStyleSourceList;
    }

    // Add a single column that fills the outline width
    NSTableColumn *column = [[NSTableColumn alloc] initWithIdentifier:@"main"];
    column.resizingMask = NSTableColumnAutoresizingMask;
    [outlineView addTableColumn:column];
    outlineView.outlineTableColumn = column;
    outlineView.columnAutoresizingStyle = NSTableViewUniformColumnAutoresizingStyle;

    outlineView.dataSource = dataSource;

    // Create delegate
    JVOutlineViewDelegate *delegate = [[JVOutlineViewDelegate alloc] init];
    delegate.callbackID = callbackID;
    outlineView.delegate = delegate;

    // Wrap in scroll view
    NSScrollView *scrollView = [[NSScrollView alloc] init];
    scrollView.translatesAutoresizingMaskIntoConstraints = NO;
    scrollView.documentView = outlineView;
    scrollView.hasVerticalScroller = YES;
    scrollView.hasHorizontalScroller = NO;
    scrollView.autohidesScrollers = YES;
    scrollView.borderType = NSNoBorder;
    scrollView.drawsBackground = NO;

    // Store references
    objc_setAssociatedObject(scrollView, kOutlineDataSourceKey, dataSource, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(scrollView, kOutlineDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    objc_setAssociatedObject(scrollView, kOutlineInnerKey, outlineView, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    // Reload and expand all
    [outlineView reloadData];
    [outlineView expandItem:nil expandChildren:YES];

    // Select initial item
    NSString *selID = [NSString stringWithUTF8String:selectedID];
    if (selID.length > 0) {
        JVOutlineItem *item = [dataSource findItemByID:selID inItems:dataSource.rootItems];
        if (item) {
            NSInteger row = [outlineView rowForItem:item];
            if (row >= 0) {
                delegate.suppressSelection = YES;
                [outlineView selectRowIndexes:[NSIndexSet indexSetWithIndex:row] byExtendingSelection:NO];
                delegate.suppressSelection = NO;
            }
        }
    }

    return (__bridge_retained void*)scrollView;
}

void JVUpdateOutlineView(void* handle, const char* dataJSON, const char* selectedID) {
    NSScrollView *scrollView = (__bridge NSScrollView*)handle;
    NSOutlineView *outlineView = objc_getAssociatedObject(scrollView, kOutlineInnerKey);
    JVOutlineDataSource *dataSource = objc_getAssociatedObject(scrollView, kOutlineDataSourceKey);
    JVOutlineViewDelegate *delegate = objc_getAssociatedObject(scrollView, kOutlineDelegateKey);
    if (!outlineView || !dataSource) return;

    // Save expanded state
    NSMutableSet<NSString*> *expandedIDs = [NSMutableSet set];
    for (NSInteger i = 0; i < outlineView.numberOfRows; i++) {
        JVOutlineItem *item = [outlineView itemAtRow:i];
        if ([outlineView isItemExpanded:item]) {
            [expandedIDs addObject:item.itemID];
        }
    }

    // Reload data
    [dataSource parseJSON:[NSString stringWithUTF8String:dataJSON]];
    [outlineView reloadData];

    // Restore expanded state — expand all by default for folders
    [outlineView expandItem:nil expandChildren:YES];

    // Update selection
    NSString *selID = [NSString stringWithUTF8String:selectedID];
    if (selID.length > 0 && delegate) {
        JVOutlineItem *item = [dataSource findItemByID:selID inItems:dataSource.rootItems];
        if (item) {
            NSInteger row = [outlineView rowForItem:item];
            if (row >= 0) {
                delegate.suppressSelection = YES;
                [outlineView selectRowIndexes:[NSIndexSet indexSetWithIndex:row] byExtendingSelection:NO];
                delegate.suppressSelection = NO;
            }
        }
    }
}
