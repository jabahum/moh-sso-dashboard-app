<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=false; section>
    <#if section = "form">
        <div class="moh-state">
            <div class="moh-state-icon moh-state-icon-error">!</div>
            <h3>${msg("errorTitle")}</h3>
            <p>
                <#if message?has_content>
                    ${kcSanitize(message.summary)?no_esc}
                <#else>
                    ${msg("unexpectedError")}
                </#if>
            </p>
            <div class="moh-actions">
                <a class="moh-primary-btn moh-button-link" href="${url.loginUrl}">
                    ${msg("backToLogin")}
                </a>
            </div>
        </div>
    </#if>
</@layout.registrationLayout>
