/**
 * REST API controllers handling HTTP requests.
 * 
 * Controllers in this package:
 * - WorkflowController: Manages workflow lifecycle and user interactions
 *   - POST /api/v1/workflow/start: Start a new workflow
 *   - POST /api/v1/workflow/user-response: Handle user responses
 *   - POST /api/v1/workflow/message: Process incoming messages
 * 
 * Controllers handle request/response mapping and delegate business
 * logic to services and workflows.
 */
package com.example.mattermost.controller; 