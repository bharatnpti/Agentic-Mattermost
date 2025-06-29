# Architecture Overview

## Project Structure

The codebase has been restructured following Domain-Driven Design (DDD) principles and clean architecture patterns for better maintainability and understanding.

### Package Organization

```
com.example.mattermost/
├── domain/                 # Core business logic
│   ├── model/             # Domain entities and value objects
│   └── repository/        # Repository interfaces
├── workflow/              # Temporal workflow layer
│   ├── activity/          # Activity interfaces
│   └── activity/impl/     # Activity implementations
├── service/               # Business services
│   └── impl/              # Service implementations
├── controller/            # REST API controllers
├── integration/           # External system integrations
│   ├── llm/              # Language Model integration
│   └── mattermost/       # Mattermost chat integration
│       └── model/        # Mattermost-specific models
├── config/                # Application configuration
└── util/                  # Utility classes
```

## Layer Responsibilities

### Domain Layer (`domain/`)
Contains the core business entities and repository interfaces. This layer is independent of external frameworks and represents the business domain.

- **model**: Core entities like `Goal`, `ActionNode`, `ActionStatus`, etc.
- **repository**: Interfaces for data persistence

### Workflow Layer (`workflow/`)
Contains Temporal workflow definitions and activities. This layer orchestrates business processes.

- **MeetingSchedulerWorkflow**: Interface defining workflow operations
- **MeetingSchedulerWorkflowImpl**: Implementation of workflow logic
- **activity**: Interfaces for workflow activities
- **activity/impl**: Concrete implementations of activities

### Service Layer (`service/`)
Contains business logic that doesn't belong in workflows or domain entities.

- **GoalExtractionService**: Extracts goals from user messages

### Controller Layer (`controller/`)
Handles HTTP requests and responses, delegating to appropriate services and workflows.

- **WorkflowController**: REST endpoints for workflow management

### Integration Layer (`integration/`)
Contains adapters for external systems, keeping them isolated from core business logic.

- **llm**: Integration with Language Learning Models (OpenAI)
- **mattermost**: Integration with Mattermost chat platform

### Configuration Layer (`config/`)
Spring configuration classes for application setup.

- **TemporalConfig**: Configures Temporal client and workers

### Utility Layer (`util/`)
Cross-cutting concerns and helper classes.

- **ApplicationContextHolder**: Static Spring context access
- **PromptHolder**: LLM prompt templates
- **InternalTools**: Helper utilities

## Design Principles

1. **Separation of Concerns**: Each package has a clear responsibility
2. **Dependency Inversion**: Higher layers depend on abstractions (interfaces) not implementations
3. **Domain Isolation**: Core business logic is isolated from frameworks and external systems
4. **Clean Architecture**: Dependencies point inward toward the domain layer

## Benefits of This Structure

1. **Maintainability**: Clear package structure makes it easy to locate and modify code
2. **Testability**: Each layer can be tested independently
3. **Flexibility**: External integrations can be easily replaced
4. **Understanding**: New developers can quickly understand the codebase organization
5. **Scalability**: New features can be added without affecting existing code 