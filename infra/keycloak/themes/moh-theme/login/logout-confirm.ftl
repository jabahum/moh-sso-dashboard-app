<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <form id="kc-logout-confirm" class="moh-form" action="${url.logoutConfirmAction}" method="post">
            <div class="moh-page-intro">
                <h3>${msg("logoutConfirmTitle")}</h3>
                <p>${msg("logoutConfirmInstruction")}</p>
            </div>

            <input type="hidden" name="session_code" value="${logoutConfirm.code}" />

            <div class="moh-actions">
                <button class="moh-primary-btn" id="kc-logout" type="submit">
                    ${msg("doLogout")}
                </button>
                <a class="moh-secondary-btn moh-button-link" href="${url.loginUrl}">
                    ${msg("doCancel")}
                </a>
            </div>
        </form>
    </#if>
</@layout.registrationLayout>
