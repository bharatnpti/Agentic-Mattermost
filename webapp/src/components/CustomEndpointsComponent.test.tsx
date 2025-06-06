import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';

import CustomEndpointsComponent from './CustomEndpointsComponent';

// Mock the ResizeObserver
class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
}
global.ResizeObserver = ResizeObserver;

describe('CustomEndpointsComponent', () => {
    const mockOnChange = jest.fn();
    const mockConfig = {
        display_name: 'Test Custom Endpoints',
        help_text: 'Manage your test endpoints.',
        // Other config properties can be added if needed by the component
    };
    const defaultProps = {
        id: 'customEndpointsSetting',
        value: '[]',
        onChange: mockOnChange,
        config: mockConfig,
    };

    beforeEach(() => {
        mockOnChange.mockClear();
    });

    test('renders correctly with empty initial value', () => {
        render(<CustomEndpointsComponent {...defaultProps} />);
        expect(screen.getByText(mockConfig.display_name)).toBeInTheDocument();
        // Help text is now part of the component structure
        expect(screen.getByText(mockConfig.help_text)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /Add Endpoint/i })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Delete/i })).not.toBeInTheDocument();
    });

    test('renders correctly with undefined initial value', () => {
        render(<CustomEndpointsComponent {...defaultProps} value={undefined as any} />);
        expect(screen.getByText(mockConfig.display_name)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /Add Endpoint/i })).toBeInTheDocument();
        expect(screen.queryByRole('button', { name: /Delete/i })).not.toBeInTheDocument();
    });

    test('renders with pre-filled data', () => {
        const initialEndpoints = [{ Name: 'Test1', Endpoint: 'http://test1.com' }];
        render(<CustomEndpointsComponent {...defaultProps} value={JSON.stringify(initialEndpoints)} />);

        expect(screen.getByLabelText('Name:')).toHaveValue('Test1');
        expect(screen.getByLabelText('Endpoint:')).toHaveValue('http://test1.com');
        expect(screen.getByRole('button', { name: /Delete endpoint Test1/i })).toBeInTheDocument();
    });

    test('adds a new endpoint when "Add Endpoint" is clicked', () => {
        render(<CustomEndpointsComponent {...defaultProps} value="[]" />);

        fireEvent.click(screen.getByRole('button', { name: /Add Endpoint/i }));

        const nameInputs = screen.getAllByLabelText('Name:');
        const endpointInputs = screen.getAllByLabelText('Endpoint:');
        expect(nameInputs.length).toBe(1);
        expect(endpointInputs.length).toBe(1);
        expect(nameInputs[0]).toHaveValue('');
        expect(endpointInputs[0]).toHaveValue('');

        expect(mockOnChange).toHaveBeenCalledTimes(1);
        expect(mockOnChange).toHaveBeenCalledWith(defaultProps.id, JSON.stringify([{ Name: '', Endpoint: '' }]));
    });

    test('deletes an endpoint', () => {
        const initialEndpoints = [{ Name: 'Test1', Endpoint: 'http://test1.com' }];
        render(<CustomEndpointsComponent {...defaultProps} value={JSON.stringify(initialEndpoints)} />);

        expect(screen.getByLabelText('Name:')).toBeInTheDocument(); // Ensure it exists before delete
        fireEvent.click(screen.getByRole('button', { name: /Delete endpoint Test1/i }));

        expect(screen.queryByLabelText('Name:')).not.toBeInTheDocument();
        expect(screen.queryByLabelText('Endpoint:')).not.toBeInTheDocument();

        expect(mockOnChange).toHaveBeenCalledTimes(1);
        expect(mockOnChange).toHaveBeenCalledWith(defaultProps.id, JSON.stringify([]));
    });

    test('edits an endpoint name', () => {
        const initialEndpoints = [{ Name: 'Test1', Endpoint: 'http://test1.com' }];
        render(<CustomEndpointsComponent {...defaultProps} value={JSON.stringify(initialEndpoints)} />);

        const nameInput = screen.getByLabelText('Name:');
        fireEvent.change(nameInput, { target: { value: 'Updated Name' } });

        expect(mockOnChange).toHaveBeenCalledTimes(1);
        expect(mockOnChange).toHaveBeenCalledWith(defaultProps.id, JSON.stringify([{ Name: 'Updated Name', Endpoint: 'http://test1.com' }]));
    });

    test('edits an endpoint URL', () => {
        const initialEndpoints = [{ Name: 'Test1', Endpoint: 'http://test1.com' }];
        render(<CustomEndpointsComponent {...defaultProps} value={JSON.stringify(initialEndpoints)} />);

        const endpointInput = screen.getByLabelText('Endpoint:');
        fireEvent.change(endpointInput, { target: { value: 'http://updated.com' } });

        expect(mockOnChange).toHaveBeenCalledTimes(1);
        expect(mockOnChange).toHaveBeenCalledWith(defaultProps.id, JSON.stringify([{ Name: 'Test1', Endpoint: 'http://updated.com' }]));
    });

    test('handles multiple endpoints and operations', () => {
        const initialEndpoints = [
            { Name: 'Service A', Endpoint: 'http://a.com' },
            { Name: 'Service B', Endpoint: 'http://b.com' },
        ];
        render(<CustomEndpointsComponent {...defaultProps} value={JSON.stringify(initialEndpoints)} />);

        // Check initial state
        let nameInputs = screen.getAllByLabelText('Name:');
        expect(nameInputs[0]).toHaveValue('Service A');
        expect(nameInputs[1]).toHaveValue('Service B');

        // Delete Service A
        fireEvent.click(screen.getByRole('button', { name: /Delete endpoint Service A/i }));
        expect(mockOnChange).toHaveBeenLastCalledWith(defaultProps.id, JSON.stringify([
            { Name: 'Service B', Endpoint: 'http://b.com' }
        ]));
        // Re-render or update state based on mock (actual component updates state internally)
        render(<CustomEndpointsComponent {...defaultProps} value={JSON.stringify([{ Name: 'Service B', Endpoint: 'http://b.com' }])} />);

        nameInputs = screen.getAllByLabelText('Name:');
        expect(nameInputs.length).toBe(1);
        expect(nameInputs[0]).toHaveValue('Service B');

        // Add a new endpoint
        fireEvent.click(screen.getByRole('button', { name: /Add Endpoint/i }));
        expect(mockOnChange).toHaveBeenLastCalledWith(defaultProps.id, JSON.stringify([
            { Name: 'Service B', Endpoint: 'http://b.com' },
            { Name: '', Endpoint: '' }
        ]));
         render(<CustomEndpointsComponent {...defaultProps} value={JSON.stringify([ { Name: 'Service B', Endpoint: 'http://b.com' }, { Name: '', Endpoint: '' }])} />);

        // Edit the new endpoint's name
        const allNameInputs = screen.getAllByLabelText('Name:');
        fireEvent.change(allNameInputs[1], { target: { value: 'Service C' } });
         expect(mockOnChange).toHaveBeenLastCalledWith(defaultProps.id, JSON.stringify([
            { Name: 'Service B', Endpoint: 'http://b.com' },
            { Name: 'Service C', Endpoint: '' }
        ]));
    });

    test('handles malformed JSON in value prop gracefully', () => {
        // Suppress console.error for this specific test as we expect an error log
        const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

        render(<CustomEndpointsComponent {...defaultProps} value="this is not json" />);

        expect(screen.getByText(mockConfig.display_name)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /Add Endpoint/i })).toBeInTheDocument();
        // No items should be rendered
        expect(screen.queryByLabelText('Name:')).not.toBeInTheDocument();

        // Check if console.error was called due to parsing failure
        expect(consoleErrorSpy).toHaveBeenCalled();

        // Restore console.error
        consoleErrorSpy.mockRestore();
    });
});
