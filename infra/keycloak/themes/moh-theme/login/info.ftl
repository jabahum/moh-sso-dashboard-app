<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <div class="moh-state">
            <div class="moh-state-icon moh-state-icon-info">i</div>
            <h3>${msg("informationTitle")}</h3>

            <#if messageHeader??>
                <p>${messageHeader}</p>
            </#if>

            <#if message?has_content>
                <p>${kcSanitize(message.summary)?no_esc}</p>
            </#if>

            <#if actionUri??>
                <div class="moh-actions">
                    <a class="moh-primary-btn moh-button-link" href="${actionUri}">
                        <#if actionUriTitle??>${actionUriTitle}<#else>${msg("doContinue")}</#if>
                    </a>
                </div>
            <#elseif client.baseUrl??>
                <div class="moh-actions">
                    <a class="moh-primary-btn moh-button-link" href="${client.baseUrl}">
                        ${msg("backToApplication")}
                    </a>
                </div>
            <#else>
                <div class="moh-actions">
                    <a class="moh-primary-btn moh-button-link" href="${url.loginUrl}">
                        ${msg("backToLogin")}
                    </a>
                </div>
            </#if>
        </div>
    </#if>
</@layout.registrationLayout>
