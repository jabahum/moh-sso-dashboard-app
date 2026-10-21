<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true displayRequiredFields=true; section>
    <#if section = "form">
        <form id="kc-register-form" class="moh-form" action="${url.registrationAction}" method="post">
            <div class="moh-page-intro">
                <h3>${msg("registerTitle")}</h3>
                <p>${msg("registerInstruction")}</p>
            </div>

            <div class="moh-field">
                <label for="firstName" class="moh-label">${msg("firstName")}</label>
                <input id="firstName" name="firstName" type="text" class="moh-input" value="${(register.formData.firstName!'')}" autocomplete="given-name" />
            </div>

            <div class="moh-field">
                <label for="lastName" class="moh-label">${msg("lastName")}</label>
                <input id="lastName" name="lastName" type="text" class="moh-input" value="${(register.formData.lastName!'')}" autocomplete="family-name" />
            </div>

            <div class="moh-field">
                <label for="email" class="moh-label">${msg("email")}</label>
                <input id="email" name="email" type="email" class="moh-input" value="${(register.formData.email!'')}" autocomplete="email" />
            </div>

            <#if !realm.registrationEmailAsUsername>
                <div class="moh-field">
                    <label for="username" class="moh-label">${msg("username")}</label>
                    <input id="username" name="username" type="text" class="moh-input" value="${(register.formData.username!'')}" autocomplete="username" />
                </div>
            </#if>

            <#if passwordRequired??>
                <div class="moh-field">
                    <label for="password" class="moh-label">${msg("password")}</label>
                    <div class="moh-password-wrap">
                        <input id="password" name="password" type="password" class="moh-input" autocomplete="new-password" />
                        <button type="button" class="moh-password-toggle" data-password-toggle data-password-target="password" aria-label="${msg("showPassword")}">
                            ${msg("showPassword")}
                        </button>
                    </div>
                </div>

                <div class="moh-field">
                    <label for="password-confirm" class="moh-label">${msg("passwordConfirm")}</label>
                    <div class="moh-password-wrap">
                        <input id="password-confirm" name="password-confirm" type="password" class="moh-input" autocomplete="new-password" />
                        <button type="button" class="moh-password-toggle" data-password-toggle data-password-target="password-confirm" aria-label="${msg("showPassword")}">
                            ${msg("showPassword")}
                        </button>
                    </div>
                </div>
            </#if>

            <#if recaptchaRequired??>
                <div class="g-recaptcha" data-size="compact" data-sitekey="${recaptchaSiteKey}"></div>
            </#if>

            <div class="moh-actions">
                <button class="moh-primary-btn" type="submit">
                    ${msg("doRegister")}
                </button>
                <a class="moh-secondary-btn moh-button-link" href="${url.loginUrl}">
                    ${msg("backToLogin")}
                </a>
            </div>
        </form>
    </#if>
</@layout.registrationLayout>
