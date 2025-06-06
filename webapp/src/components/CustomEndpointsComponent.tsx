import React, { useState, useEffect } from 'react';

interface Endpoint {
    Name: string;
    Endpoint: string;
}

interface Props {
    id: string;
    value: string; // JSON string
    onChange: (id: string, value: string) => void;
    config: any; // Configuration object from plugin.json, specifically settings_schema.settings item
    // `label` and `helpText` are also available but derived from config.display_name and config.help_text
    // `disabled` can also be passed.
}

const CustomEndpointsComponent: React.FC<Props> = ({ id, value, onChange, config }) => {
    const [endpoints, setEndpoints] = useState<Endpoint[]>([]);

    useEffect(() => {
        try {
            // Attempt to parse the value prop, default to empty array if it's falsy (e.g., empty string)
            const parsedEndpoints = value ? JSON.parse(value) : [];
            setEndpoints(parsedEndpoints);
        } catch (error) {
            console.error("Error parsing endpoints JSON:", error);
            // If JSON is invalid, default to an empty array
            setEndpoints([]);
            // Optionally, call onChange to clear out invalid data from the config
            // onChange(id, JSON.stringify([]));
        }
    }, [value]); // Rerun effect if the `value` prop changes

    const handleAddEndpoint = () => {
        const newEndpoints = [...endpoints, { Name: "", Endpoint: "" }];
        setEndpoints(newEndpoints);
        onChange(id, JSON.stringify(newEndpoints));
    };

    const handleDeleteEndpoint = (index: number) => {
        const newEndpoints = endpoints.filter((_, i) => i !== index);
        setEndpoints(newEndpoints);
        onChange(id, JSON.stringify(newEndpoints));
    };

    const handleInputChange = (index: number, field: keyof Endpoint, inputValue: string) => {
        const newEndpoints = endpoints.map((endpoint, i) =>
            i === index ? { ...endpoint, [field]: inputValue } : endpoint
        );
        setEndpoints(newEndpoints);
        onChange(id, JSON.stringify(newEndpoints));
    };

    // Use config properties for display elements
    const displayName = config?.display_name || "Custom Endpoints";
    const helpText = config?.help_text || "";

    return (
        // Mattermost setting components are typically wrapped in a div with a specific structure
        // or rely on the Admin Console's rendering of label and help text.
        // This component will be rendered as the *input* part of a setting.
        // The `label` (display_name) and `help_text` are usually rendered by the Admin Console itself.
        // However, a sub-header for the component itself can be useful.
        <div className="CustomEndpointsComponent">
            {/* Optional: Display help text if not automatically handled by Admin Console for custom components */}
            {helpText && <p className="help-text">{helpText}</p>}

            {endpoints.map((endpoint, index) => (
                <div key={index} style={{
                    marginBottom: '10px',
                    border: '1px solid var(--center-channel-color-16)', // Use Mattermost theme variables
                    padding: '10px',
                    borderRadius: '4px' // Consistent with Mattermost UI
                }}>
                    <div style={{ marginBottom: '5px' }}>
                        <label htmlFor={`${id}_name_${index}`} style={{ fontWeight: '600' }}>Name:</label>
                        <input
                            id={`${id}_name_${index}`}
                            type="text"
                            value={endpoint.Name}
                            onChange={(e) => handleInputChange(index, "Name", e.target.value)}
                            className="form-control" // Standard Mattermost class for inputs
                            placeholder="e.g., My Service"
                        />
                    </div>
                    <div style={{ marginBottom: '5px' }}>
                        <label htmlFor={`${id}_endpoint_${index}`} style={{ fontWeight: '600' }}>Endpoint:</label>
                        <input
                            id={`${id}_endpoint_${index}`}
                            type="text"
                            value={endpoint.Endpoint}
                            onChange={(e) => handleInputChange(index, "Endpoint", e.target.value)}
                            className="form-control" // Standard Mattermost class for inputs
                            placeholder="e.g., http://localhost:8000/api"
                        />
                    </div>
                    <button
                        onClick={() => handleDeleteEndpoint(index)}
                        className="btn btn-danger btn-sm" // Standard Mattermost button classes
                        aria-label={`Delete endpoint ${endpoint.Name || index + 1}`}
                    >
                        Delete
                    </button>
                </div>
            ))}
            <button
                onClick={handleAddEndpoint}
                className="btn btn-primary" // Standard Mattermost button class
                style={{ marginTop: '10px' }}
            >
                Add Endpoint
            </button>
        </div>
    );
};

export default CustomEndpointsComponent;
