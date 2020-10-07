SHELL := bash

HASHICORP_RELEASES = https://releases.hashicorp.com
HASHIWANTALL =  terraform-provider-azurerm/2.29.0 \
		  		terraform/0.12.29

current_dir := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

TFBINDIR	= $(current_dir)/terraform

## Find the 'highest' vM.m.p version tag if there is one, else
#  use the commit ID
GITTAGS	    := $(shell git tag --points-at HEAD | sort -V)
GITVER		:= $(patsubst v%,%,$(filter v%, $(GITTAGS)))
MAXVER		:= $(lastword $(GITVER))
ISDIRTY		:= $(shell git diff-index --quiet HEAD -- || echo yes)

ifndef MAXVER
 MAXVER = commit-$(shell git rev-parse --short HEAD)
endif

## But always use 'dev' if there are changes in the index
ifdef ISDIRTY
  MAXVER = dev
endif

.PHONY: build fmtcheck

default: build

build: fmtcheck
	go install -ldflags '-X github.com/jake-scott/apim-tools/version.Version=$(MAXVER)'

fmtcheck:
	@"$(CURDIR)/scripts/gofmtcheck.sh"

acctest: tfget
	TERRAFORM=$(TFBINDIR)/terraform scripts/acctests.sh


#####   BEGIN HASHICORP DOWNLOAD SECTION    ############
GOHOSTOS   = $(shell go env GOHOSTOS)
GOHOSTARCH = $(shell go env GOHOSTARCH)
HASHI_GPG_KEY = 91A6E7F85D05C65630BEF18951852D87348FFC4C

export GNUPGHOME=./.gnupg


$(GNUPGHOME)/trustdb.gpg:
	@mkdir -p $(GNUPGHOME)
	@chmod go= $(GNUPGHOME)
	gpg --import hashicorp-gpg-key.asc

.PHONY: tfget
tfget: $(HASHIWANTALL)

.PHONY: $(HASHIWANTALL)
$(HASHIWANTALL):
	@$(MAKE) tfdownload HASHIPRODUCT=$(subst /,,$(dir $@)) HASHIVER=$(notdir $@)

## Target used as a sub-make to grab a release from the Hashicorp downloads site
HASHI_ZIP_NAME = $(HASHIPRODUCT)_$(HASHIVER)_$(GOHOSTOS)_$(GOHOSTARCH).zip
HASHI_SUM_NAME = $(HASHIPRODUCT)_$(HASHIVER)_SHA256SUMS
HASHI_DL_FILE = download/$(HASHI_ZIP_NAME)
HASHI_SUM_FILE = download/$(HASHI_SUM_NAME)

## Download the distro then check the signature on the checksum file, then verify
#  the checksums 
.PHONY: tfdownload
tfdownload: $(GNUPGHOME)/trustdb.gpg $(HASHI_DL_FILE) $(HASHI_SUM_FILE)
	@gpg --batch --verify $(HASHI_SUM_FILE).sig $(HASHI_SUM_FILE)
	@cd download && sha256sum -c <(grep $(HASHI_ZIP_NAME) $(HASHI_SUM_NAME))
	@mkdir -p $(TFBINDIR)
	@unzip -od $(TFBINDIR) $(HASHI_DL_FILE)

$(HASHI_DL_FILE):
	@echo Downloading $(HASHIPRODUCT) version $(HASHIVER)
	@mkdir -p download
	curl -s -o $@ $(HASHICORP_RELEASES)/$(HASHIPRODUCT)/$(HASHIVER)/$(HASHI_ZIP_NAME)

$(HASHI_SUM_FILE):
	@echo Downloading $(HASHIPRODUCT) checksum files
	@mkdir -p download
	curl -s -o $@ $(HASHICORP_RELEASES)/$(HASHIPRODUCT)/$(HASHIVER)/$(HASHI_SUM_NAME)
	curl -s -o $@.sig $(HASHICORP_RELEASES)/$(HASHIPRODUCT)/$(HASHIVER)/$(HASHI_SUM_NAME).sig

## Use this target to refresh the GPG key from the key server
hashicorp-gpg-key.asc:
	mkdir -p $(GNUPGHOME)
	chmod go= $(GNUPGHOME)
	gpg --batch --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys $(HASHI_GPG_KEY)
	gpg -a --export $(HASHI_GPG_KEY) >$@

#####   END HASHICORP DOWNLOAD SECTION    ############


