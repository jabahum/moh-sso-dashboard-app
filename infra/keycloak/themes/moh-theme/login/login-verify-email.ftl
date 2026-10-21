<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <div class="moh-state">
            <div class="moh-state-icon moh-state-icon-info">i</div>
            <h3>${msg("emailVerifyTitle")}</h3>
            <p>${msg("emailVerifyInstruction1", user.email!'')}</p>
            <p>${msg("emailVerifyInstruction2")}</p>

            <form id="kc-verify-email-form" class="moh-form" action="${url.loginAction}" method="post">
                <div class="moh-actions">
                    <button class="moh-primary-btn" type="submit">
                        ${msg("doClickHere")}
                    </button>
                </div>
            </form>
        </div>
    </#if>
</@layout.registrationLayout>
