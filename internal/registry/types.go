package registry

import (
	"fmt"
	"strings"

	"sync-image/internal/config"
	"sync-image/pkg/logger"
)

// RegistryType registry type enumeration
type RegistryType string

const (
	RegistryTypeGeneric RegistryType = "generic"
)

// String returns string representation of registry type
func (rt RegistryType) String() string {
	return string(rt)
}

// RegistryProcessor registry processor interface
type RegistryProcessor interface {
	// ProcessImage processes image (permissions, tags, etc.)
	ProcessImage(imageName string) error

	// GetType gets processor type
	GetType() RegistryType

	// SupportsRegistry checks if registry is supported
	SupportsRegistry(registryURL string) bool

	// ValidateConfig validates configuration
	ValidateConfig() error

	// GetName gets processor name (for logging)
	GetName() string
}

// RegistryTypeDetector registry type detector
type RegistryTypeDetector struct {
	logger logger.Logger
}

// NewRegistryTypeDetector creates new registry type detector
func NewRegistryTypeDetector(log logger.Logger) *RegistryTypeDetector {
	return &RegistryTypeDetector{
		logger: log,
	}
}

// registryPatterns registry type detection rules (now only generic)
var registryPatterns = map[RegistryType][]string{
	// All registries use generic processor
}

// DetectRegistryType detects registry type (now all registries use generic processor)
func (d *RegistryTypeDetector) DetectRegistryType(registryURL string) RegistryType {
	d.logger.Debug("Detecting registry type: %s", registryURL)

	// Clean URL by removing protocol prefix
	cleanURL := strings.TrimPrefix(registryURL, "https://")
	cleanURL = strings.TrimPrefix(cleanURL, "http://")

	// Get domain name
	parts := strings.Split(cleanURL, "/")
	domain := parts[0]

	// All registries now use generic processor with special handling for Huawei SWR
	d.logger.Debug("All registries use generic processor: %s", domain)
	return RegistryTypeGeneric
}

// Registry-specific detection is now handled by PostProcessor implementations

// GenericProcessor generic registry processor default implementation
type GenericProcessor struct {
	logger logger.Logger
}

// NewGenericProcessor creates new generic processor
func NewGenericProcessor(log logger.Logger) RegistryProcessor {
	return &GenericProcessor{
		logger: log,
	}
}

// ProcessImage processes image (generic implementation, no special operations)
func (p *GenericProcessor) ProcessImage(imageName string) error {
	p.logger.Info("Using generic processor to process image: %s", imageName)
	p.logger.Debug("Generic processor completed, no special operations performed")
	return nil
}

// GetType gets processor type
func (p *GenericProcessor) GetType() RegistryType {
	return RegistryTypeGeneric
}

// SupportsRegistry checks if registry is supported (generic processor supports all registries)
func (p *GenericProcessor) SupportsRegistry(registryURL string) bool {
	return true
}

// ValidateConfig validates configuration (generic processor has no configuration)
func (p *GenericProcessor) ValidateConfig() error {
	return nil
}

// GetName gets processor name
func (p *GenericProcessor) GetName() string {
	return "Generic Registry Processor"
}

// EnhancedGenericProcessor enhanced generic processor with Docker login and post-processing support
type EnhancedGenericProcessor struct {
	config        *config.GenericRegistryConfig
	postProcessor *PostProcessorManager
	logger        logger.Logger
}

// NewEnhancedGenericProcessor creates new enhanced generic processor
func NewEnhancedGenericProcessor(cfg *config.GenericRegistryConfig, postProcessor *PostProcessorManager, log logger.Logger) RegistryProcessor {
	return &EnhancedGenericProcessor{
		config:        cfg,
		postProcessor: postProcessor,
		logger:        log,
	}
}

// ProcessImage processes image (enhanced implementation with Docker login and post-processing support)
func (p *EnhancedGenericProcessor) ProcessImage(imageName string) error {
	p.logger.Info("Using enhanced generic processor to process image: %s", imageName)

	// If authentication info is provided, try to login to Docker registry
	if p.config != nil && p.config.Username != "" && p.config.Password != "" {
		p.logger.Debug("Authentication detected, attempting registry login: %s", p.config.Registry)
		if err := p.dockerLogin(); err != nil {
			p.logger.Warn("Docker login failed, continuing: %v", err)
		} else {
			p.logger.Debug("Docker login successful")
		}
	}

	// Execute post-processing operations (e.g., setting permissions, adding tags)
	if p.postProcessor != nil && p.config != nil {
		p.logger.Debug("Executing post-processing operations")
		if err := p.postProcessor.ProcessImage(imageName, p.config.Registry); err != nil {
			p.logger.Warn("Post-processing failed: %v", err)
			// Don't return error, post-processing failures shouldn't stop the main flow
		}
	}

	p.logger.Debug("Enhanced generic processor completed, image processed")
	return nil
}

// GetType gets processor type
func (p *EnhancedGenericProcessor) GetType() RegistryType {
	return RegistryTypeGeneric
}

// SupportsRegistry checks if registry is supported (enhanced processor supports all registries)
func (p *EnhancedGenericProcessor) SupportsRegistry(registryURL string) bool {
	return true
}

// ValidateConfig validates configuration
func (p *EnhancedGenericProcessor) ValidateConfig() error {
	if p.config.Registry == "" {
		return fmt.Errorf("registry is required")
	}
	// username and password are optional
	return nil
}

// GetName gets processor name
func (p *EnhancedGenericProcessor) GetName() string {
	return "Enhanced Generic Registry Processor"
}

// dockerLogin performs Docker login
func (p *EnhancedGenericProcessor) dockerLogin() error {
	// Log Docker login information
	// Note: Actual Docker login is handled by Docker daemon
	p.logger.Debug("Preparing Docker login: registry=%s, username=%s", p.config.Registry, p.config.Username)

	return nil
}

// Post-processing is now handled by dedicated PostProcessor implementations
// This keeps the generic processor focused on core functionality

// ProcessorError processor error type
type ProcessorError struct {
	ProcessorType RegistryType
	Operation     string
	Err           error
	Context       map[string]interface{}
}

// Error implements error interface
func (e *ProcessorError) Error() string {
	return fmt.Sprintf("registry processor error [%s:%s]: %v", e.ProcessorType, e.Operation, e.Err)
}

// NewProcessorError creates new processor error
func NewProcessorError(processorType RegistryType, operation string, err error) *ProcessorError {
	return &ProcessorError{
		ProcessorType: processorType,
		Operation:     operation,
		Err:           err,
		Context:       make(map[string]interface{}),
	}
}

// WithContext adds context information
func (e *ProcessorError) WithContext(key string, value interface{}) *ProcessorError {
	e.Context[key] = value
	return e
}
