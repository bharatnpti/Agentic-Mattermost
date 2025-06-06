// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType);

    // Add the method for registering custom admin console settings
    registerAdminConsoleCustomSetting(key: string, component: React.ComponentType<any>, options?: {showTitle: boolean});

    // Add more if needed from https://developers.mattermost.com/extend/plugins/webapp/reference
}
