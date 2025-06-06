// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useEffect} from 'react';

interface Endpoint {
    Name: string;
    Endpoint: string;
}

interface CustomEndpointSettingProps {
    id: string;
    label: string;
    helpText: string;
    value: string; // JSON string of Endpoint[]
    disabled: boolean;
    config: any; // Mattermost config object
    license: any; // Mattermost license object
    setByEnv: boolean;
    onChange: (id: string, value: any) => void;
    onError: (error: string | null) => void; // Added to handle validation
}

const CustomEndpointSetting: React.FC<CustomEndpointSettingProps> = ({
    id,
    value,
    onChange,
    onError, // Added to handle validation
}) => {
    const [endpoints, setEndpoints] = useState<Endpoint[]>([]);

    useEffect(() => {
        try {
            if (value && value.trim() !== '') {
                const parsedEndpoints = JSON.parse(value);
                if (Array.isArray(parsedEndpoints)) {
                    setEndpoints(parsedEndpoints);
                } else {
                    setEndpoints([]);
                    onError?.('Invalid format: Value is not an array.');
                }
            } else {
                setEndpoints([]);
            }
            onError?.(null); // Clear error on successful parse or empty value
        } catch (error) {
            // console.error('Error parsing CustomEndpoints JSON:', error);
            setEndpoints([]);
            onError?.('Invalid JSON format.');
        }
    }, [value, onError]);

    const handleAddEndpoint = () => {
        setEndpoints([...endpoints, {Name: '', Endpoint: ''}]);

        // onChange will be called by the effect below
    };

    const handleRemoveEndpoint = (index: number) => {
        const newEndpoints = endpoints.filter((_, i) => i !== index);
        setEndpoints(newEndpoints);

        // onChange will be called by the effect below
    };

    const handleChangeEndpoint = (index: number, field: keyof Endpoint, fieldValue: string) => {
        const newEndpoints = endpoints.map((ep, i) =>
            (i === index ? ({...ep, [field]: fieldValue}) : ep), // Added parentheses for no-confusing-arrow
        );
        setEndpoints(newEndpoints);

        // onChange will be called by the effect below
    };

    // Effect to call onChange when endpoints array changes
    useEffect(() => {
        try {
            const jsonValue = JSON.stringify(endpoints);
            onChange(id, jsonValue);
            onError?.(null); // Clear error on successful update
        } catch (error) {
            // console.error('Error stringifying CustomEndpoints JSON:', error);
            // Potentially inform Mattermost of this error if a mechanism exists beyond console
            onError?.('Error preparing data for save.');
        }
    }, [endpoints, id, onChange, onError]);

    // Basic styling (inline for now, can be moved to CSS modules or SCSS later)
    const styles = {
        container: {marginBottom: '10px'},
        endpointItem: {
            border: '1px solid #ddd',
            padding: '10px',
            marginBottom: '10px',
            borderRadius: '4px',
        },
        input: {
            width: 'calc(50% - 20px)', // Adjust width as needed
            padding: '8px',
            marginBottom: '5px',
            marginRight: '5px', // Spacing between inputs
            border: '1px solid #ccc',
            borderRadius: '4px',
        },
        button: {
            padding: '8px 15px',
            backgroundColor: '#007bff',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: 'pointer',
            marginRight: '5px',
        },
        removeButton: {
            backgroundColor: '#dc3545',
        },
    };

    return (
        <div>
            {endpoints.map((endpoint, index) => (
                <div
                    key={index}
                    style={styles.endpointItem}
                >
                    <input
                        type='text'
                        placeholder='Name'
                        value={endpoint.Name}
                        onChange={(e) => handleChangeEndpoint(index, 'Name', e.target.value)}
                        style={styles.input}
                    />
                    <input
                        type='text'
                        placeholder='Endpoint URL'
                        value={endpoint.Endpoint}
                        onChange={(e) => handleChangeEndpoint(index, 'Endpoint', e.target.value)}
                        style={styles.input}
                    />
                    <button
                        type='button'
                        onClick={() => handleRemoveEndpoint(index)}
                        style={{...styles.button, ...styles.removeButton}}
                    >
                        {'Remove'}
                    </button>
                </div>
            ))}
            <button
                type='button'
                onClick={handleAddEndpoint}
                style={styles.button}
            >
                {'Add Endpoint'}
            </button>
        </div>
    );
};

export default CustomEndpointSetting;
