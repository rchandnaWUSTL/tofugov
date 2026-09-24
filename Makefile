TOFU ?= tofu
WORKSPACES := $(sort $(wildcard fixtures/workspaces/ws-*))
UPGRADE_FLAGS := --provider hashicorp/random=3.9.1 --provider hashicorp/local=2.9.1 --provider hashicorp/null=3.3.2

.PHONY: build test seed reset demo

build:
	go build -o bin/tofugov ./cmd/tofugov

test:
	go test ./...

# Apply each fixture at its old pinned provider versions. seed.tfvars makes the
# applied state differ from the current config, so the upgrade run surfaces
# pending changes the way real bulk upgrades do.
seed:
	@for ws in $(WORKSPACES); do \
	  echo "seeding $$ws"; \
	  vf=""; [ -f $$ws/seed.tfvars ] && vf="-var-file=seed.tfvars"; \
	  (cd $$ws && $(TOFU) init -input=false -no-color >/dev/null && \
	    $(TOFU) apply -auto-approve -input=false -no-color $$vf >/dev/null) || exit 1; \
	done

# Restore pre-upgrade .tf files and delete generated fixture state. The change
# history in .tofugov/tofugov.db is kept.
reset:
	@for ws in $(WORKSPACES); do \
	  if [ -d $$ws/.tofugov/backup ]; then cp $$ws/.tofugov/backup/*.tf $$ws/; fi; \
	  rm -rf $$ws/.terraform $$ws/.terraform.lock.hcl $$ws/terraform.tfstate $$ws/terraform.tfstate.backup $$ws/.tofugov $$ws/out; \
	done

demo: build reset seed
	./bin/tofugov upgrade $(UPGRADE_FLAGS) $(WORKSPACES)
