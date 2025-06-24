// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useEffect} from 'react';

export type EndpointType = 'arc' | 'workflow';

export const EndpointTypeArc: EndpointType = 'arc';
export const EndpointTypeWorkflow: EndpointType = 'workflow';

export interface CustomEndpoint {
    Name: string;
    Endpoint: string;
    Type: EndpointType; // Added Type field
}

interface CustomEndpointSettingProps {
    id: string;
    label: string;
    helpText: string;
    value: CustomEndpoint[]; // Array of CustomEndpoint objects, not JSON string
    disabled: boolean;
    config: any; // Mattermost config object
    license: any; // Mattermost license object
    setByEnv: boolean;
    onChange: (id: string, value: CustomEndpoint[]) => void;
    onError: (error: string | null) => void;
}

const CustomEndpointSetting: React.FC<CustomEndpointSettingProps> = ({
    id,
    value,
    onChange,
    onError,
    disabled = false,
}) => {
    const [endpoints, setEndpoints] = useState<CustomEndpoint[]>([]);

    useEffect(() => {
        try {
            if (Array.isArray(value)) {
                const processedEndpoints = value.map((ep) => {
                    if (ep && typeof ep === 'object' && 'Name' in ep && 'Endpoint' in ep) {
                        return {
                            Name: ep.Name,
                            Endpoint: ep.Endpoint,
                            Type: ep.Type || EndpointTypeArc, // Default to Arc if Type is missing
                        };
                    }
                    return null; // Invalid endpoint structure
                }).filter((ep): ep is CustomEndpoint => ep !== null); // Type guard to filter out nulls

                setEndpoints(processedEndpoints);

                const originalValidCount = value.filter((ep) => ep && typeof ep === 'object' && 'Name' in ep && 'Endpoint' in ep).length;
                const typesDefaultedCount = value.filter((ep) => ep && typeof ep === 'object' && 'Name' in ep && 'Endpoint' in ep && !ep.Type).length;

                if (processedEndpoints.length !== originalValidCount) {
                    onError?.('Some endpoint entries were malformed and have been filtered out.');
                } else if (typesDefaultedCount > 0) {
                    // Using a less alarming message if only types were defaulted
                    onError?.(`Some endpoints had their type defaulted to '${EndpointTypeArc}'.`);
                } else {
                    onError?.(null);
                }
            } else { // Handle non-array cases here
                setEndpoints([]);
                if (value === null || value === undefined) {
                    onError?.(null);
                } else {
                    onError?.('Invalid format: Value is not an array.');
                }
            }
        } catch (error) {
            setEndpoints([]);
            onError?.('Error loading endpoints.');
        }
    }, [value, onError]);

    const handleAddEndpoint = () => {
        if (disabled) {
            return;
        }
        // Add new endpoint with a default Type
        const newEndpoints = [...endpoints, {Name: '', Endpoint: '', Type: EndpointTypeArc}];
        setEndpoints(newEndpoints);
        onChange(id, newEndpoints);
    };

    const handleRemoveEndpoint = (index: number) => {
        if (disabled) {
            return;
        }
        const newEndpoints = endpoints.filter((_, i) => i !== index);
        setEndpoints(newEndpoints);
        onChange(id, newEndpoints);
    };

    const handleChangeEndpoint = (index: number, field: keyof CustomEndpoint, fieldValue: string | EndpointType) => {
        if (disabled) {
            return;
        }
        const newEndpoints = endpoints.map((ep, i) => {
            if (i === index) {
                return {...ep, [field]: fieldValue};
            }
            return ep;
        });
        setEndpoints(newEndpoints);
        onChange(id, newEndpoints);
    };

    const validateEndpoints = (endpointList: CustomEndpoint[]): string | null => {
        // Check for duplicate names
        const names = endpointList.map((ep) => ep.Name.trim()).filter((name) => name !== '');
        const uniqueNames = new Set(names);
        if (names.length !== uniqueNames.size) {
            return 'Duplicate endpoint names are not allowed.';
        }

        // Check for invalid URLs
        for (const endpoint of endpointList) {
            if (endpoint.Name.trim() && endpoint.Endpoint.trim()) {
                try {
                    // eslint-disable-next-line no-new
                    new URL(endpoint.Endpoint); // Suppressing 'no-new' as it's used for validation
                } catch {
                    return `Invalid URL format for "${endpoint.Name}": ${endpoint.Endpoint}`;
                }
            }
        }

        return null;
    };

    // Validate on every change
    useEffect(() => {
        const validationError = validateEndpoints(endpoints);
        onError?.(validationError);
    }, [endpoints, onError]);

    // Styling
    const styles = {
        container: {
            marginBottom: '20px',
            fontFamily: 'Arial, sans-serif',
        },
        header: {
            marginBottom: '15px',
            fontSize: '14px',
            fontWeight: '600' as const,
            color: '#333',
        },
        endpointItem: {
            display: 'flex' as const,
            alignItems: 'center' as const,
            gap: '10px',
            padding: '12px',
            marginBottom: '10px',
            border: '1px solid #e1e5e9',
            borderRadius: '6px',
            backgroundColor: disabled ? '#f8f9fa' : '#ffffff',
        },
        inputGroup: {
            display: 'flex' as const,
            flex: '1',
            gap: '10px',
        },
        input: {
            flex: '1',
            padding: '8px 12px',
            border: '1px solid #d1d5db',
            borderRadius: '4px',
            fontSize: '14px',
            backgroundColor: disabled ? '#f3f4f6' : '#ffffff',
            color: disabled ? '#6b7280' : '#374151',
        },
        nameInput: {
            maxWidth: '200px',
        },
        button: {
            padding: '8px 16px',
            backgroundColor: '#1f2937',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: disabled ? 'not-allowed' : 'pointer',
            fontSize: '14px',
            fontWeight: '500' as const,
            opacity: disabled ? 0.6 : 1,
        },
        addButton: {
            backgroundColor: '#059669',
            marginTop: '10px',
        },
        removeButton: {
            backgroundColor: '#dc2626',
            padding: '8px 12px',
        },
        emptyState: {
            textAlign: 'center' as const,
            padding: '20px',
            color: '#6b7280',
            fontSize: '14px',
            fontStyle: 'italic' as const,
        },
        example: {
            fontSize: '12px',
            color: '#6b7280',
            marginTop: '5px',
            fontStyle: 'italic' as const,
        },
    };

    return (
        <div style={styles.container}>
            <div style={styles.header}>
                {'Custom Endpoints'} {/* Fixed: react/jsx-no-literals */}
                <div style={styles.example}>
                    {'Example: Name "weather" → Endpoint "ws://weather:8080.com"'} {/* Fixed: react/jsx-no-literals & react/no-unescaped-entities */}
                </div>
            </div>

            {endpoints.length === 0 ? (
                <div style={styles.emptyState}>
                    {'No endpoints configured. Click "Add Endpoint" to get started.'} {/* Fixed: react/jsx-no-literals & react/no-unescaped-entities */}
                </div>
            ) : (
                endpoints.map((endpoint, index) => (
                    <div
                        key={index}
                        style={styles.endpointItem}
                    >
                        <div style={styles.inputGroup}>
                            <input
                                type='text'
                                placeholder='Name (e.g., weather)'
                                value={endpoint.Name}
                                onChange={(e) => handleChangeEndpoint(index, 'Name', e.target.value)}
                                style={{...styles.input, ...styles.nameInput}}
                                disabled={disabled}
                            />
                            <input
                                type='text'
                                placeholder='Endpoint URL (e.g., ws://weather:8080.com)'
                                value={endpoint.Endpoint}
                                onChange={(e) => handleChangeEndpoint(index, 'Endpoint', e.target.value)}
                                style={styles.input}
                                disabled={disabled}
                            />
                            <select
                                value={endpoint.Type || EndpointTypeArc} // Default to Arc if type is somehow undefined
                                onChange={(e) => handleChangeEndpoint(index, 'Type', e.target.value as EndpointType)}
                                style={styles.input} // Assuming styles.input is appropriate for select
                                disabled={disabled}
                            >
                                <option value={EndpointTypeArc}>{'Arc'}</option>
                                <option value={EndpointTypeWorkflow}>{'Workflow'}</option>
                            </select>
                        </div>
                        <button
                            type='button'
                            onClick={() => handleRemoveEndpoint(index)}
                            style={{...styles.button, ...styles.removeButton}}
                            disabled={disabled}
                            title='Remove this endpoint'
                        >
                            {'Remove'} {/* Fixed: react/jsx-no-literals */}
                        </button>
                    </div>
                ))
            )}

            <button
                type='button'
                onClick={handleAddEndpoint}
                style={{...styles.button, ...styles.addButton}}
                disabled={disabled}
            >
                {'Add Endpoint'} {/* Fixed: react/jsx-no-literals */}
            </button>
        </div>
    );
};

export default CustomEndpointSetting;
