package template_test

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
	"github.com/gatblau/release-engine/internal/module/infra/template/fragments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func engineTestParams() *template.ProvisionParams {
	return &template.ProvisionParams{
		ContractVersion:    "v1",
		RequestName:        "checkout-prod",
		Tenant:             "payments",
		Owner:              "platform-team",
		Environment:        "production",
		WorkloadProfile:    "medium",
		TemplateName:       catalog.K8sAppName,
		CompositionRef:     "composition-web-v1",
		Namespace:          "platform-system",
		Residency:          "eu",
		PrimaryRegion:      "eu-west-1",
		SecondaryRegion:    "eu-central-1",
		Availability:       "high",
		DataClassification: "confidential",
		IngressMode:        "public",
		EgressMode:         "nat",
		DRRequired:         true,
		BackupRequired:     true,
		Kubernetes:         template.KubernetesParams{Enabled: true},
	}
}

func TestEngine_Render_IncludesAlwaysOnFragments(t *testing.T) {
	p := engineTestParams()
	p.ExtraTags = map[string]string{"owner-team": "core"}

	engine := template.NewEngine(
		&fragments.TagsFragment{},
		&fragments.ComplianceFragment{},
	)

	out, err := engine.Render(p)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))

	spec := doc["spec"].(map[string]any)
	params := spec["parameters"].(map[string]any)
	assert.Contains(t, params, "tags")
	assert.Contains(t, params, "compliance")
}

func TestEngine_Render_RequiresAtLeastOneCapability(t *testing.T) {
	p := engineTestParams()
	p.Kubernetes.Enabled = false

	engine := template.NewEngine(&fragments.TagsFragment{})
	_, err := engine.Render(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one infrastructure capability must be enabled")
}

func TestEngine_Render_CrossValidationMutualExclusion(t *testing.T) {
	p := engineTestParams()
	p.Kubernetes.Enabled = true
	p.Kubernetes.Tier = "standard"
	p.Kubernetes.Size = "medium"
	p.VM.Enabled = true
	p.VM.Count = 1
	p.VM.InstanceFamily = "general"
	p.VM.Size = "medium"
	p.VM.OSFamily = "linux"

	engine := template.NewEngine(&fragments.TagsFragment{})
	_, err := engine.Render(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot enable both kubernetes and virtual machines")
}

func TestEngine_Render_CrossValidationCriticalRequiresObservability(t *testing.T) {
	p := engineTestParams()
	p.Availability = "critical"
	p.Observability.Enabled = false
	p.Kubernetes.Tier = "standard"
	p.Kubernetes.Size = "medium"

	engine := template.NewEngine(&fragments.TagsFragment{})
	_, err := engine.Render(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "observability must be enabled for critical availability")
}

func TestEngine_Render_WithCatalogAppliesDefaults(t *testing.T) {
	p := engineTestParams()
	p.CompositionRef = ""
	p.Kubernetes.Tier = ""
	p.Kubernetes.Size = "medium"

	cats, err := catalog.LoadAll()
	require.NoError(t, err)

	engine := template.NewEngineWithCatalog(
		cats,
		&fragments.TagsFragment{},
		&fragments.ComplianceFragment{},
		&fragments.KubernetesFragment{},
	)

	out, err := engine.Render(p)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(out, &doc))

	spec := doc["spec"].(map[string]any)
	compRef := spec["compositionRef"].(map[string]any)
	assert.Equal(t, "composition-k8s-application-v1", compRef["name"])

	params := spec["parameters"].(map[string]any)
	kube := params["kubernetes"].(map[string]any)
	assert.Equal(t, "standard", kube["tier"])
}

func TestEngine_Render_CatalogForbiddenCapability(t *testing.T) {
	p := engineTestParams()
	p.CompositionRef = ""
	p.VM.Enabled = true
	p.VM.Count = 1
	p.VM.InstanceFamily = "general"
	p.VM.Size = "medium"
	p.VM.OSFamily = "linux"

	cats, err := catalog.LoadAll()
	require.NoError(t, err)

	engine := template.NewEngineWithCatalog(cats, &fragments.TagsFragment{})
	_, err = engine.Render(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
	assert.Contains(t, err.Error(), "vm")
}

func TestEngine_Render_CatalogRequiredCapability(t *testing.T) {
	p := engineTestParams()
	p.TemplateName = catalog.DataProcName
	p.CompositionRef = ""
	p.Kubernetes.Enabled = true
	p.Kubernetes.Tier = "advanced"
	p.Kubernetes.Size = "medium"
	p.ObjectStore.Enabled = false
	p.Messaging.Enabled = false

	cats, err := catalog.LoadAll()
	require.NoError(t, err)

	engine := template.NewEngineWithCatalog(cats, &fragments.TagsFragment{})
	_, err = engine.Render(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
	assert.Contains(t, err.Error(), "object_storage")
}
