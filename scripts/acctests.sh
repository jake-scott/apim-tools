#!/usr/bin/env bash

set -e

## Delete resources created by Terraform and remove tmp dir, 
#  unless NOCLEAN is set
cleanup() {
    if [[ -n ${NOCLEAN} ]]
    then
        echo Not cleaning, tmp dir ${TMPDIR}
        return
    fi

    if [[ -e ${TFDIR}/terraform.tfstate ]]
    then
        echo Deleting Azure resources
        pushd ${TFDIR}
        ${TERRAFORM} destroy -auto-approve ${TFDIR}
        popd
    fi

    rm -rfv ${TMPDIR}
}

configure_tf() {
    ## Configure terraform
    TFDIR=${TMPDIR}/tf
    mkdir -p ${TFDIR}
    cp -f ${MYDIR}/../t/test.tf ${TFDIR}
    
    export TF_VAR_region=${az_region:?Missing variable}
    export TF_VAR_rg_name=apimtooltest-${RND}
    export TF_VAR_rand=${RND}
    export ARM_CLIENT_ID=${az_client_id:?Missing variable}
    export ARM_CLIENT_SECRET=${az_client_secret:?Missing variable}
    export ARM_SUBSCRIPTION_ID=${az_subscription:?Missing variable}
    export ARM_TENANT_ID=${az_tenant:?Missing variable}
    TERRAFORM=${TERRAFORM:-terraform}
}

create_resources() {
    pushd ${TFDIR}

    ${TERRAFORM} init && \
    ${TERRAFORM} plan -out=tf.plan && \
    ${TERRAFORM} apply -auto-approve tf.plan
    ret=$?

    popd

    return $ret
}

publish_test_site() {
    archive=$1

    echo "------- Uploading test portal $archive"
    ./apim-tools devportal upload \
        --subscription 0f32bf3b-84fb-487c-81d9-df6ccaae74aa \
        --rg apimtooltest-${RND} \
        --apim test-${RND} \
        --in ${MYDIR}/../t/${archive}  || return 1

    echo '------- Publishing portal'
    ./apim-tools devportal publish \
        --subscription 0f32bf3b-84fb-487c-81d9-df6ccaae74aa \
        --rg apimtooltest-${RND} \
        --apim test-${RND} \
        --wait || return 1

    return 0
}

test_live_site1() {
    echo '------- Testing the uploaded portal'
    DPURL=$(./apim-tools devportal endpoints \
        --subscription 0f32bf3b-84fb-487c-81d9-df6ccaae74aa \
        --rg apimtooltest-${RND} \
        --apim test-${RND} \
        --json \
        | jq -r .devPortalUrl)

    echo "  -- Fetching ${DPURL}"

    CODE=$(curl -s -o ${TMPDIR}/mainpage.out -w '%{http_code}' "${DPURL}")
    if [[ $CODE -ne 200 ]]
    then
        echo Failed to fetch site, got HTTP code $CODE
        cat ${TMPDIR}/mainpage.out
        return 1
    fi
    echo "  -- Downloaded main page OK"

    if ! grep -q 'Welcome to Cat World' ${TMPDIR}/mainpage.out
    then
        echo Failed to find 'Welcome to Cat World' in the home page
        return 1
    fi

    echo "  -- Found modification on main page, good"

    CODE=$(curl -s -o ${TMPDIR}/cat.out -w '%{http_code}' "${DPURL}/content/cat1.jpg")
    if [[ $CODE -ne 200 ]]
    then
        echo Failed to fetch cat picture, got HTTP code $CODE
        cat ${TMPDIR}/cat.out
        return 1
    fi
    echo "  -- Found cat picture, super good"

    echo "Test passed!"

    return 0
}

test_live_site2() {
    echo '------- Testing the uploaded portal'
    DPURL=$(./apim-tools devportal endpoints \
        --subscription 0f32bf3b-84fb-487c-81d9-df6ccaae74aa \
        --rg apimtooltest-${RND} \
        --apim test-${RND} \
        --json \
        | jq -r .devPortalUrl)

    echo "  -- Fetching ${DPURL}"

    CODE=$(curl -s -o ${TMPDIR}/mainpage.out -w '%{http_code}' "${DPURL}")
    if [[ $CODE -ne 200 ]]
    then
        echo Failed to fetch site, got HTTP code $CODE
        cat ${TMPDIR}/mainpage.out
        return 1
    fi
    echo "  -- Downloaded main page OK"

    if grep -q 'Welcome to Cat World' ${TMPDIR}/mainpage.out
    then
        echo 'Welcome to Cat World' is still on the home page - bad
        return 1
    fi

    echo "  -- Cat world is no more, sad but correct"

    CODE=$(curl -s -o ${TMPDIR}/cat.out -w '%{http_code}' "${DPURL}/content/cat1.jpg")
    if [[ $CODE -ne 404 ]]
    then
        echo Expected 404 not found fetching cat picture, got HTTP code $CODE
        return 1
    fi
    echo "  -- Cat picture is gone, this is also correct"


    if ! grep -q 'Welcome to Dog World' ${TMPDIR}/mainpage.out
    then
        echo Failed to find 'Dog World' in the home page
        return 1
    fi
    echo "  -- Found modification on main page, good"

    CODE=$(curl -s -o ${TMPDIR}/dog.out -w '%{http_code}' "${DPURL}/content/dog-superman.jpg")
    if [[ $CODE -ne 200 ]]
    then
        echo Failed to fetch dog superman picture, got HTTP code $CODE
        cat ${TMPDIR}/dog.out
        return 1
    fi
    echo "  - Found cute dog photo, awesome"

    echo "Test passed!"

    return 0
}


echo "==> Running acceptance tests"

## Temp work directory, in memory
TMPDIR=${TMPDIR:-$(mktemp -d -p /dev/shm  apim-tools-acctests-XXXXXXX)}

## Make sure we clean up when we exit
trap cleanup exit

MYDIR=$(dirname $0)
RND=${RAND:-$(dd if=/dev/urandom count=1 bs=5 2>/dev/null| base32)}

configure_tf                 || exit 1
create_resources             || exit 1

publish_test_site test1.zip  || exit 1
test_live_site1              || exit 1

publish_test_site test2.zip  || exit 1
test_live_site2              || exit 1

exit 0

