<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <form id="kc-reset-password-form" class="moh-form" action="${url.loginAction}" method="post">
            <div class="moh-page-intro">
                <h3>${msg("resetPasswordTitle")}</h3>
                <p>${msg("resetPasswordInstruction")}</p>
            </div>

            <div class="moh-field">
                <label for="username" class="moh-label">
                    <#if !realm.loginWithEmailAllowed>
                        ${msg("username")}
                    <#else>
                        ${msg("usernameOrEmail")}
                    </#if>
                </label>
                <input
                    id="username"
                    name="username"
                    type="text"
                    class="moh-input"
                    value="${(auth.attemptedUsername!'')}"
                    autocomplete="username"
                    autofocus
                />
            </div>

            <div class="moh-actions">
                <button class="moh-primary-btn" type="submit">
                    ${msg("doSubmit")}
                </button>
                <a class="moh-secondary-btn moh-button-link" href="${url.loginUrl}">
                    ${msg("backToLogin")}
                </a>
            </div>
        </form>
    </#if>
</@layout.registrationLayout>
