<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <div class="moh-state">
            <div class="moh-state-icon moh-state-icon-info">i</div>
            <h3>${msg("pageExpiredTitle")}</h3>
            <p>${msg("pageExpiredInstruction")}</p>

            <div class="moh-actions moh-actions-split">
                <a class="moh-primary-btn moh-button-link" href="${url.loginRestartFlowUrl}">
                    ${msg("restartLogin")}
                </a>
                <a class="moh-secondary-btn moh-button-link" href="${url.loginUrl}">
                    ${msg("backToLogin")}
                </a>
            </div>
        </div>
    </#if>
</@layout.registrationLayout>
